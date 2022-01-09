package Cache

import "testing"

func TestCache(t *testing.T) {
	cache, _ := NewCache()

	if err := cache.Put("a", 1); err != nil {
		t.Error("Cannot put a value into Cache instance")
	}

	value, err := cache.Get("a")
	if err != nil{
		t.Error("Cannot get value from Cache instance")
	}
	if value != 1 {
		t.Error("Expected value 1, actual: ", value)
	}
}
