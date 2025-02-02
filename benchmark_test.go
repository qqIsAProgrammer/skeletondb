package skeleton_test

import (
	"math/rand"
	skeleton "skeletondb"
	"strconv"
	"sync"
	"testing"
)

func intToKey(i int) []byte {
	return []byte(strconv.Itoa(i))
}

func randKey() []byte {
	return []byte(strconv.Itoa(rand.Int()))
}

func BenchmarkPutSeq1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		k := intToKey(i)
		db.Put(k, k)
	}
}
func BenchmarkPutSeq10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 10; j++ {
			k := intToKey(i)
			db.Put(k, k)
		}
	}
}
func BenchmarkPutSeq100(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 100; j++ {
			k := intToKey(i)
			db.Put(k, k)
		}
	}
}
func BenchmarkPutSeq1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 1000; j++ {
			k := intToKey(i)
			db.Put(k, k)
		}
	}
}
func BenchmarkPutSeq10000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 10000; j++ {
			k := intToKey(i)
			db.Put(k, k)
		}
	}
}
func BenchmarkPutSeq100000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 100000; j++ {
			k := intToKey(i)
			db.Put(k, k)
		}
	}
}
func BenchmarkPutRand1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		k := randKey()
		db.Put(k, k)
	}
}
func BenchmarkPutRand10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 10; j++ {
			k := randKey()
			db.Put(k, k)
		}
	}
}
func BenchmarkPutRand100(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 100; j++ {
			k := randKey()
			db.Put(k, k)
		}
	}
}
func BenchmarkPutRand1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 1000; j++ {
			k := randKey()
			db.Put(k, k)
		}
	}
}
func BenchmarkPutRand10000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 10000; j++ {
			k := randKey()
			db.Put(k, k)
		}
	}
}
func BenchmarkPutRand100000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, _ := skeleton.NewDB(nil)
		for j := 0; j < 100000; j++ {
			k := randKey()
			db.Put(k, k)
		}
	}
}

func BenchmarkPutRand10000Parallel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var done sync.WaitGroup
		db, _ := skeleton.NewDB(nil)
		for i := 0; i < 10; i++ {
			done.Add(1)
			go func() {
				for j := 0; j < 1000; j++ {
					k := randKey()
					db.Put(k, k)
				}
				done.Done()
			}()
		}
		done.Wait()
	}
}

func BenchmarkPutRand100000Parallel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var done sync.WaitGroup
		db, _ := skeleton.NewDB(nil)
		for i := 0; i < 10; i++ {
			done.Add(1)
			go func() {
				for j := 0; j < 10000; j++ {
					k := randKey()
					db.Put(k, k)
				}
				done.Done()
			}()
		}
		done.Wait()
	}
}
