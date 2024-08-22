package sqlited

type Cfg struct {
	Path string `json:"path" yaml:"path" validate:"required" `
}

func NewCfg() *Cfg {
	return &Cfg{}
}
