package upd

type Cfg struct {
	DeployCmd []string `yaml:"deploy_cmd"`
	StartCmd  []string `yaml:"start_cmd"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}
