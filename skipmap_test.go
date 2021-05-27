package skipmap

import (
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zhangyunhao116/fastrand"
)

func TestSkipMap(t *testing.T) {
	m := NewInt()

	// Correctness.
	m.Store(123, "123")
	v, ok := m.Load(123)
	if !ok || v != "123" || m.Len() != 1 {
		t.Fatal("invalid")
	}

	m.Store(123, "456")
	v, ok = m.Load(123)
	if !ok || v != "456" || m.Len() != 1 {
		t.Fatal("invalid")
	}

	m.Store(123, 456)
	v, ok = m.Load(123)
	if !ok || v != 456 || m.Len() != 1 {
		t.Fatal("invalid")
	}

	m.Delete(123)
	_, ok = m.Load(123)
	if ok || m.Len() != 0 {
		t.Fatal("invalid")
	}

	_, ok = m.LoadOrStore(123, 456)
	if ok || m.Len() != 1 {
		t.Fatal("invalid")
	}

	v, ok = m.Load(123)
	if !ok || v != 456 || m.Len() != 1 {
		t.Fatal("invalid")
	}

	v, ok = m.LoadAndDelete(123)
	if !ok || v != 456 || m.Len() != 0 {
		t.Fatal("invalid")
	}

	_, ok = m.LoadOrStore(123, 456)
	if ok || m.Len() != 1 {
		t.Fatal("invalid")
	}

	m.LoadOrStore(456, 123)
	if ok || m.Len() != 2 {
		t.Fatal("invalid")
	}

	m.Range(func(key int, value interface{}) bool {
		if key == 123 {
			m.Store(123, 123)
		} else if key == 456 {
			m.LoadAndDelete(456)
		}
		return true
	})

	v, ok = m.Load(123)
	if !ok || v != 123 || m.Len() != 1 {
		t.Fatal("invalid")
	}

	// Concurrent.
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		i := i
		wg.Add(1)
		go func() {
			m.Store(i, int(i+1000))
			wg.Done()
		}()
	}
	wg.Wait()
	wg.Add(1)
	go func() {
		m.Delete(600)
		wg.Done()
	}()
	wg.Wait()
	wg.Add(1)
	var count int64
	go func() {
		m.Range(func(key int, val interface{}) bool {
			atomic.AddInt64(&count, 1)
			return true
		})
		wg.Done()
	}()
	wg.Wait()

	val, ok := m.Load(500)
	if !ok || reflect.TypeOf(val).Kind().String() != "int" || val.(int) != 1500 {
		t.Fatal("fail")
	}

	_, ok = m.Load(600)
	if ok {
		t.Fatal("fail")
	}

	if m.Len() != 999 || int(count) != m.Len() {
		t.Fatal("fail")
	}
	// Correctness 2.
	var m1 sync.Map
	m2 := NewUint32()
	var v1, v2 interface{}
	var ok1, ok2 bool
	for i := 0; i < 100000; i++ {
		rd := fastrand.Uint32n(10)
		r1, r2 := fastrand.Uint32n(100), fastrand.Uint32n(100)
		if rd == 0 {
			m1.Store(r1, r2)
			m2.Store(r1, r2)
		} else if rd == 1 {
			v1, ok1 = m1.LoadAndDelete(r1)
			v2, ok2 = m2.LoadAndDelete(r1)
			if ok1 != ok2 || v1 != v2 {
				t.Fatal(rd, v1, ok1, v2, ok2)
			}
		} else if rd == 2 {
			v1, ok1 = m1.LoadOrStore(r1, r2)
			v2, ok2 = m2.LoadOrStore(r1, r2)
			if ok1 != ok2 || v1 != v2 {
				t.Fatal(rd, v1, ok1, v2, ok2, "input -> ", r1, r2)
			}
		} else if rd == 3 {
			m1.Delete(r1)
			m2.Delete(r1)
		} else if rd == 4 {
			m2.Range(func(key uint32, value interface{}) bool {
				v, ok := m1.Load(key)
				if !ok || v != value {
					t.Fatal(v, ok, key, value)
				}
				return true
			})
		} else {
			v1, ok1 = m1.Load(r1)
			v2, ok2 = m2.Load(r1)
			if ok1 != ok2 || v1 != v2 {
				t.Fatal(rd, v1, ok1, v2, ok2)
			}
		}
	}
}

func TestSkipMapDesc(t *testing.T) {
	m := NewIntDesc()
	cases := []int{10, 11, 12}
	for _, v := range cases {
		m.Store(v, nil)
	}
	i := len(cases) - 1
	m.Range(func(key int, value interface{}) bool {
		if key != cases[i] {
			t.Fail()
		}
		i--
		return true
	})
}

/* Test from sync.Map */
func TestConcurrentRange(t *testing.T) {
	const mapSize = 1 << 10

	m := NewInt64()
	for n := int64(1); n <= mapSize; n++ {
		m.Store(n, int64(n))
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		close(done)
		wg.Wait()
	}()
	for g := int64(runtime.GOMAXPROCS(0)); g > 0; g-- {
		r := rand.New(rand.NewSource(g))
		wg.Add(1)
		go func(g int64) {
			defer wg.Done()
			for i := int64(0); ; i++ {
				select {
				case <-done:
					return
				default:
				}
				for n := int64(1); n < mapSize; n++ {
					if r.Int63n(mapSize) == 0 {
						m.Store(n, n*i*g)
					} else {
						m.Load(n)
					}
				}
			}
		}(g)
	}

	iters := 1 << 10
	if testing.Short() {
		iters = 16
	}
	for n := iters; n > 0; n-- {
		seen := make(map[int64]bool, mapSize)

		m.Range(func(ki int64, vi interface{}) bool {
			k, v := ki, vi.(int64)
			if v%k != 0 {
				t.Fatalf("while Storing multiples of %v, Range saw value %v", k, v)
			}
			if seen[k] {
				t.Fatalf("Range visited key %v twice", k)
			}
			seen[k] = true
			return true
		})

		if len(seen) != mapSize {
			t.Fatalf("Range visited %v elements of %v-element Map", len(seen), mapSize)
		}
	}
}
