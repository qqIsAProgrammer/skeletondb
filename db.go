package skeleton

import (
	"bytes"
	"log"
	"os"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	rootPage = pageID(1)
)

var zeroTime = time.Unix(0, 0)

// DB is a skeletondb instance.
type DB struct {
	pages  *[]*delta
	closed chan struct{}
	config Config

	// largestPageID is automatically incremented when a new page is created.
	largestPageID int64
	pageIDPool    chan pageID

	// Queues
	splitQueue       chan pageID
	consolidateQueue chan pageID
}

type unsafeDB struct {
	pages unsafe.Pointer
}

// NewDB creates a new database.
func NewDB(c *Config) (*DB, error) {
	log.SetOutput(os.Stderr)
	if c == nil {
		c = &DefaultConfig
	}
	if err := c.Verify(); err != nil {
		return nil, err
	}
	db := &DB{
		splitQueue:       make(chan pageID, 10),
		consolidateQueue: make(chan pageID, 10),
		closed:           make(chan struct{}),
		pages: &[]*delta{
			{
				next: &delta{
					page: &page{id: 1},
				},
			},
		},
		config:        *c,
		largestPageID: 1,
		pageIDPool:    make(chan pageID, 10),
	}
	go db.workerLoop()
	return db, nil
}

// nextPageID returns the next available pageID, either from the pool, or by
// incrementing the largestPageID.
func (db *DB) nextPageID() pageID {
	var id pageID
	select {
	case id = <-db.pageIDPool:
	default:
		id = pageID(atomic.AddInt64(&db.largestPageID, 1))
	}

	for old := db.pages; int(id) > len(*old); {
		nLen := len(*old) * 2
		if nLen == 0 {
			nLen = 1
		}
		new := make([]*delta, nLen)
		copy(new, *old)
		for i := len(*old); i < nLen; i++ {
			new[i] = &delta{}
		}

		unsafeDB := (*unsafeDB)(unsafe.Pointer(db))
		if atomic.CompareAndSwapPointer(&unsafeDB.pages, unsafe.Pointer(old), unsafe.Pointer(&new)) {
			break
		}
	}
	return id
}

// Close closes the database and all workers.
func (db *DB) Close() {
	close(db.closed)
}

// Key represents a single key with potentially multiple values. A key with no
// values represents a read intent.
type key struct {
	key    []byte
	txn    *Txn
	values []value
	read   bool // read is whether this key is a get intent
}

func (k key) clone() key {
	k.values = append([]value{}, k.values...)
	return k
}

// byKey implements sort.Interface for []*key first based on key and then by
// the timestamp.
type byKey []*key

func (a byKey) Len() int      { return len(a) }
func (a byKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byKey) Less(i, j int) bool {
	compare := bytes.Compare(a[i].key, a[j].key)
	if compare == 0 {
		return a[i].values[0].time.Before(a[j].values[0].time)
	}
	return compare < 0
}

func (k *key) getAt(txn *Txn, at time.Time) ([]byte, bool) {
	if k.txn != nil && txn != txn && k.txn.status != StatusCommitted {
		return nil, false
	}
	for _, v := range k.values {
		if at != zeroTime && at.Before(v.time) {
			continue
		}
		if v.tombstone {
			return nil, true
		}
		return v.value, true
	}
	return nil, false
}

// Value represents a value and the previous versions.
type value struct {
	value     []byte
	time      time.Time
	tombstone bool
}

type pageID int64

type page struct {
	id    pageID
	key   []byte
	keys  []*key
	left  pageID
	right pageID
}

// Get gets a value from the database.
func (db *DB) Get(key []byte) ([]byte, bool) {
	return db.getAt(nil, key, zeroTime)
}

// GetAt gets a value from the database at the specified time.
func (db *DB) GetAt(key []byte, at time.Time) ([]byte, bool) {
	return db.getAt(nil, key, at)
}

func (db *DB) getAt(txn *Txn, k []byte, at time.Time) ([]byte, bool) {
	id := rootPage
	page := db.getPage(id)
	delta := page.next
	deltaCount := 0
	defer func() {
		// If there is a pending transaction, abort the current transaction.
		if txn != nil {
			if t := page.hasPendingTxn(k); t != nil && t != txn {
				if err := txn.Close(); err != nil {
					panic("txn should not already be closed")
				}
			} else {
				if err := db.putKey(&key{
					key:  k,
					txn:  txn,
					read: true,
				}); err != nil {
					panic("error adding read intent")
				}
			}
		}

		// Check if the depth is too high, and if so, queue for consolidation.
		if deltaCount > db.config.MaxDeltaCount {
			db.consolidateQueue <- id
		}
	}()
	for delta != nil {
		if (delta.key == nil) == (delta.page == nil) {
			panic("invariant: exactly one of delta.key, delta.page must be set")
		}

		if delta.page != nil { // Check page for match.
			dPage := delta.page
			if dPage.key != nil { // Index node
				if bytes.Compare(dPage.key, k) <= 0 {
					id = dPage.right
				} else {
					id = dPage.left
				}
				page = db.getPage(id)
				delta = page.next
			} else { // Data node
				for _, entry := range dPage.keys {
					if bytes.Equal(entry.key, k) {
						return entry.getAt(txn, at)
					}
				}
				break
			}
		} else if delta.key != nil { // Check delta for match.
			deltaCount++

			// Skip uncommitted keys.
			t := delta.key.txn
			if t != nil && t != txn && t.status != StatusCommitted {
				delta = delta.next
				continue
			}
			if bytes.Equal(k, delta.key.key) {
				// If the time isn't found in the delta, look at older data.
				if v, ok := delta.key.getAt(txn, at); ok {
					return v, true
				}
			}
			delta = delta.next
		}
	}
	return nil, false
}

// Put writes a value into the database.
func (db *DB) Put(k, v []byte) error {
	return db.put(nil, k, v)
}

func (db *DB) put(txn *Txn, k, v []byte) error {
	return db.putKey(&key{
		key: k,
		txn: txn,
		values: []value{
			{
				value: v,
				time:  time.Now(),
			},
		},
	})
}

// Delete removes a value from the database.
func (db *DB) Delete(k []byte) error {
	return db.delete(nil, k)
}

func (db *DB) delete(txn *Txn, k []byte) error {
	return db.putKey(&key{
		key: k,
		txn: txn,
		values: []value{
			{
				tombstone: true,
				time:      time.Now(),
			},
		},
	})
}

func (db *DB) putKey(key *key) error {
	for {
		id := rootPage
		page := db.getPage(id)
		d := page.next

		// Find the matching page.  We only have to check if the first page is nil
		// since index nodes won't have any deltas on top of them.
		for d.page != nil {
			if d.page.key == nil {
				break
			}

			if bytes.Compare(d.page.key, key.key) <= 0 {
				id = d.page.right
			} else {
				id = d.page.left
			}
			d = db.getPage(id).next
		}

		// Check for pending transactions on the same key.
		if txn := d.hasPendingTxn(key.key); txn != nil && txn != key.txn {
			if err := key.txn.Close(); err != nil {
				return err
			}
			return ErrTxnConflict
		}

		insert := delta{
			key:  key,
			next: d,
		}
		if db.savePageNext(id, d, &insert) {
			break
		}
	}
	return nil
}

func (db *DB) getPage(id pageID) *delta {
	return (*db.pages)[id-1]
}

func (db *DB) savePageNext(id pageID, old, new *delta) bool {
	page := db.getPage(id)
	unsafePage := (*unsafeDelta)(unsafe.Pointer(page))
	return atomic.CompareAndSwapPointer(&unsafePage.next, unsafe.Pointer(old), unsafe.Pointer(new))
}

func (db *DB) getDeltaCount(id pageID) int {
	return db.getPage(id).deltaCount()
}
