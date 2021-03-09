package award

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.binatex.com/platform/hydra/pb"
	"gitlab.binatex.com/platform/hydra/util/redis"
)

const (
	totalHashKey    = "account:total_state"
	totalStatsTTL   = time.Hour * 2
	persistInterval = time.Minute * 10
)

type AccountTotalState struct {
	Totals     []*pb.AccountBuffTotalState
	LastUpdate time.Time
}

func (a *AccountTotalState) GetProfitMax() float64 {
	for _, total := range a.Totals {
		if total.BuffType == pb.BuffTypeSteppedMaxProfitPerDayLimit {
			return total.TotalAttribute.Money
		}
	}
	return 0
}

func NewTotalState(ctx context.Context, logger *logrus.Entry, reCli *redis.Client) (*TotalState, error) {
	out := &TotalState{
		ctx:    ctx,
		logger: logger,
		reCli:  reCli,
		states: make(map[int64]*AccountTotalState),
	}
	err := out.load()
	if err != nil {
		return nil, err
	}
	go func() {
		ticker := time.NewTicker(persistInterval)
		for {
			select {
			case <-out.ctx.Done():
				return
			case <-ticker.C:
				err = out.persist()
				if err != nil {
					out.logger.Error(err)
				}
			}
		}
	}()
	return out, nil
}

// TotalState is a typical realization of Redis wrapper with mutexed state to work && periodically dumping
type TotalState struct {
	sync.RWMutex

	ctx    context.Context
	logger *logrus.Entry
	reCli  *redis.Client

	betMin     float64
	betMax     float64
	profitMax  float64
	optionMax  int32
	validTypes []pb.BuffType

	states map[int64]*AccountTotalState
}

func (s *TotalState) Start(validTypes []pb.BuffType, betMin float64, betMax float64, profitMax float64, optionMax int32) error {
	s.betMax = betMax
	s.betMin = betMin
	s.profitMax = profitMax
	s.optionMax = optionMax
	s.validTypes = validTypes
	return nil
}

func (s *TotalState) InitAccountState(accountID int64, accountEffects []*pb.AccountBuff) *AccountTotalState {
	parsedABs := make(map[pb.BuffType]*pb.BuffAttribute)
	for _, buffType := range s.validTypes {
		parsedABs[buffType] = new(pb.BuffAttribute)
		for _, ab := range accountEffects {
			if buffType == ab.Buff.Type {
				err := s.sumAttrs(parsedABs[buffType], ab.Buff.Attr)
				if err != nil {
					s.logger.Errorln(err)
				}
			}
		}
	}
	parsedABs[pb.BuffTypeSteppedMinBetLimit].Money += s.betMin
	parsedABs[pb.BuffTypeSteppedMaxBetLimit].Money += s.betMax
	parsedABs[pb.BuffTypeSteppedMaxProfitPerDayLimit].Money += s.profitMax
	parsedABs[pb.BuffTypeSteppedMaxParallelOpenOptionLimit].MaxFactor += int64(s.optionMax)

	s.Lock()
	defer s.Unlock()

	state, exists := s.states[accountID]
	if !exists {
		s.states[accountID] = &AccountTotalState{
			Totals:     make([]*pb.AccountBuffTotalState, 0, len(parsedABs)),
			LastUpdate: time.Now().UTC(),
		}
		state = s.states[accountID]
	}
	for k, v := range parsedABs {
		state.Totals = append(state.Totals, &pb.AccountBuffTotalState{BuffType: k, TotalAttribute: v})
	}
	return state
}

func (s *TotalState) Update(accountID int64, effects []*pb.AccountBuff) (*AccountTotalState, error) {
	s.Lock()
	defer s.Unlock()

	state := s.states[accountID]
	state.LastUpdate = time.Now().UTC()
	for _, totalState := range state.Totals {
		for _, effect := range effects {
			// TODO : Maybe add check for CFG.EffectTypes 'cos of new Effect.Type add to 'Totals' ^_^
			if totalState.BuffType == effect.Buff.Type {
				err := s.sumAttrs(totalState.TotalAttribute, effect.Buff.Attr)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	}
	return state, nil
}

// INNER LOGIC

func (s *TotalState) sumAttrs(total, delta *pb.BuffAttribute) error {
	total.Money += delta.Money
	total.Multiplier += delta.Multiplier
	total.EqualityFactor += delta.EqualityFactor
	total.Percent += delta.Percent
	total.Divider += delta.Divider
	total.MaxFactor += delta.MaxFactor
	total.MinFactor += delta.MinFactor
	if len(delta.Duration) > 0 {
		add, err := time.ParseDuration(delta.Duration)
		if err != nil {
			return err
		}
		if len(total.Duration) > 0 {
			base, err := time.ParseDuration(total.Duration)
			if err != nil {
				return err
			}
			total.Duration = (base + add).String()
		} else {
			total.Duration = add.String()
		}
	}
	return nil
}

func (s *TotalState) load() error {
	valString, err := s.reCli.Get(totalHashKey).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(valString), s.states)
}

func (s *TotalState) persist() error {
	s.Lock()
	defer s.Unlock()

	for accountID, state := range s.states {
		if time.Now().UTC().Sub(state.LastUpdate) > totalStatsTTL {
			delete(s.states, accountID)
		}
	}
	data, err := json.Marshal(s.states)
	if err != nil {
		s.logger.Error(err)
		return nil
	}
	_, err = s.reCli.Set(totalHashKey, string(data), 0).Result()
	if err != nil {
		return err
	}
	return nil
}
