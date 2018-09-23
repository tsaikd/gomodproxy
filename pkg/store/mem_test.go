package store

import (
	"context"
	"math/rand"
	"testing"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	m := Memory(t.Log, -1)
	m.Put(ctx, Snapshot{Module: "foo", Version: "v1.0.0", Data: []byte("hello")})
	m.Put(ctx, Snapshot{Module: "bar", Version: "v1.0.0", Data: []byte{}})
	m.Put(ctx, Snapshot{Module: "baz", Version: "v1.0.0", Data: []byte("world")})
	if res, err := m.Get(ctx, "foo", "v1.0.0"); err != nil {
		t.Fatal(err)
	} else if res.Module != "foo" || res.Version != "v1.0.0" || string(res.Data) != "hello" {
		t.Fatal(res)
	}
}

func TestMemoryStoreOverflow(t *testing.T) {
	ctx := context.Background()
	m := Memory(t.Log, 10)
	m.Put(ctx, Snapshot{Module: "foo", Version: "v1.0.0", Data: make([]byte, 4)})
	m.Put(ctx, Snapshot{Module: "bar", Version: "v1.0.0", Data: make([]byte, 7)})

	// "foo" should be removed, because adding "bar" exceeds the capacity
	if res, err := m.Get(ctx, "foo", "v1.0.0"); err == nil {
		t.Fatal(res)
	} else if _, err := m.Get(ctx, "bar", "v1.0.0"); err != nil {
		t.Fatal(err)
	}

	m.Put(ctx, Snapshot{Module: "baz", Version: "v1.0.0", Data: make([]byte, 3)})

	// both "bar" and "baz" should be in store
	if _, err := m.Get(ctx, "bar", "v1.0.0"); err != nil {
		t.Fatal(err)
	} else if _, err := m.Get(ctx, "baz", "v1.0.0"); err != nil {
		t.Fatal(err)
	}

	m.Get(ctx, "bar", "v1.0.0")
	m.Put(ctx, Snapshot{Module: "qux", Version: "v1.0.0", Data: make([]byte, 3)})

	// "bar" should remain in store, since it was accessed recently, "baz" should be removed
	if _, err := m.Get(ctx, "bar", "v1.0.0"); err != nil {
		t.Fatal(err)
	} else if res, err := m.Get(ctx, "baz", "v1.0.0"); err == nil {
		t.Fatal(res)
	}
}

func TestMemoryStoreRandom(t *testing.T) {
	snaphots := []Snapshot{
		Snapshot{Module: "a", Version: "v1.0.0", Data: make([]byte, 1)},
		Snapshot{Module: "b", Version: "v1.0.0", Data: make([]byte, 3)},
		Snapshot{Module: "c", Version: "v1.0.0", Data: make([]byte, 5)},
		Snapshot{Module: "d", Version: "v1.0.0", Data: make([]byte, 7)},
		Snapshot{Module: "e", Version: "v1.0.0", Data: make([]byte, 11)},
		Snapshot{Module: "f", Version: "v1.0.0", Data: make([]byte, 13)},
	}

	m := Memory(t.Log, 12)
	for i := 0; i < 100; i++ {
		ctx := context.Background()
		if rand.Int()%5 > 2 {
			m.Put(ctx, snaphots[rand.Intn(len(snaphots))])
		} else {
			m.Get(ctx, snaphots[rand.Intn(len(snaphots))].Module, "v1.0.0")
		}
	}
}
