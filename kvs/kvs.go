package kvs

type KVS[K comparable, V any] interface {
	Store(K, V)
	Load(K) (V, bool)
	LoadOrStore(K, V) (V, bool)
	LoadAndDelete(K) (V, bool)
	Delete(K)
	Range(func(K, V) bool)
}
