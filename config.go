package award

import (
	"time"

	"go.uber.org/fx"

	"gitlab.binatex.com/platform/hydra/pb"
	"gitlab.binatex.com/platform/hydra/util/app"
)

func NewFXConfig() (FXConfig, error) {
	cfg := new(FXConfig)
	err := app.ReadConfig(cfg, app.ReadCfgPathFlag())
	if err != nil {
		return FXConfig{}, err
	}
	return *cfg, nil
}

// FXConfig is a Uber FX lib (dependency injection) wrapper
type FXConfig struct {
	fx.Out
	Config *Config `json:"config" yaml:"config"`
}

// Config is representation of cfg file
type Config struct {
	EffectConfig        *EffectConfig        `json:"effect_config" yaml:"effect_config"`
	ProfitConfig        *ConfigProfit        `json:"profit_config" yaml:"profit_config"`
	PeriodicAwardConfig *PeriodicAwardConfig `json:"periodic_award_config" yaml:"periodic_award_config"`
}

type ConfigProfit struct {
	ResetLimitTime  time.Duration `json:"reset_limit_time" yaml:"reset_limit_time"`
	ResetLimitValue float64       `json:"reset_limit_value" yaml:"reset_limit_value"`
}

type EffectConfig struct {
	AccumTypes      []pb.BuffType     `json:"accum_types" yaml:"accum_types"`
	WatchCategories []pb.BuffCategory `json:"watch_categories" yaml:"watch_categories"`
	SequenceAwards  []*SequenceAward  `json:"sequence_awards" yaml:"sequence_awards"`
}

type SequenceAward struct {
	WinNum int64 `json:"win_num" yaml:"win_num"`
	BuffID int64 `json:"buff_id" yaml:"buff_id"`
}

type PeriodicAwardConfig struct {
	Coefficient             float64       `json:"coefficient" yaml:"coefficient"`
	MinLevel                int64         `json:"min_level" yaml:"min_level"`
	PeriodStart             time.Duration `json:"period_start" yaml:"period_start"`
	PeriodContinuous        time.Duration `json:"period_continuous" yaml:"period_continuous"`
	PromoTimestampReduction time.Duration `json:"promo_timestamp_reduction" yaml:"promo_timestamp_reduction"`
	PromoPurchaseLimit      int64         `json:"promo_purchase_limit" yaml:"promo_purchase_limit"`
	PromoOptionLimit        int64         `json:"promo_option_limit" yaml:"promo_option_limit"`
}

func (c *Config) GetMaxWinNum() int64 {
	var out int64 = 0
	for _, v := range c.EffectConfig.SequenceAwards {
		if v.WinNum > out {
			out = v.WinNum
		}
	}
	return out
}
