package option

import "testing"

type sampleCfg struct {
	Host string
	Port int
}

func TestApply(t *testing.T) {
	cfg := Apply(&sampleCfg{Host: "localhost", Port: 8080},
		nil,
		func(c *sampleCfg) { c.Host = "0.0.0.0" },
		func(c *sampleCfg) { c.Port = 9000 },
	)
	if cfg.Host != "0.0.0.0" || cfg.Port != 9000 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
