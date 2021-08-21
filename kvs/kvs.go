package kvs

type Type string

type Cfg struct {
	Type Type        `yaml:"type" json:"type"`
	Cfg  interface{} `yaml:"cfg"  json:"cfg"`
}

type KVS interface {
	Store(k string, v any)
	StoreAsString(k string, v any)
	Load(k string) (any, bool)
	LoadOrStore(k string, v any) (any, bool)
	LoadAndDelete(k string) (any, bool)
	Del(k string)
	LoadAsBool(k string) bool
	LoadAsString(k string) string
	LoadAsStringOr(k string, d string) string
	LoadAsInt(k string) int
	LoadAsIntOr(k string, d int) int
	LoadAsUint(k string) uint
	LoadAsUintOr(k string, d uint) uint
	LoadAsFloat(k string) float64
	LoadAsFloatOr(k string, d float64) float64
	LoadTo(k string, to any) error
	Collect() map[string]any
	CollectAsString() map[string]string
	Range(func(k string, v any) bool)
}
