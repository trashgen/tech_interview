package award

import (
	"fmt"
	"time"

	"gitlab.binatex.com/platform/hydra/models/dbRedis"
	"gitlab.binatex.com/platform/hydra/pb"
	"gitlab.binatex.com/platform/hydra/util/queue"
)

func NewProfitLimiter(mediator *Mediator) (*ProfitLimitObserver, error) {
	dsProfit, err := dbRedis.NewProfitLimitState(mediator.ctx, mediator.logger, mediator.reCli)
	if err != nil {
		return nil, err
	}
	out := &ProfitLimitObserver{
		Mediator: mediator,
		dsProfit: dsProfit,
	}
	return out, nil
}

// ProfitLimitObserver checks when to start decreasing limit to 0. Starts after some user actions and then do it
// periodically till the end
type ProfitLimitObserver struct {
	*Mediator

	// Typical Redis state in file 'redis_account_total_state.go'
	dsProfit *dbRedis.ProfitLimitState
}

func (l *ProfitLimitObserver) Start() error {
	states := l.dsProfit.GetOutdated()
	for _, state := range states {
		// TODO : Decrease all outdated from Redis
	}
	return nil
}

func (l *ProfitLimitObserver) InitAccount(accountID int64, profitMax float64) (*dbRedis.AccountProfitLimitState, error) {
	state := l.dsProfit.InitAccountState(accountID, profitMax)
	err := l.publishAccountProfitLimitUpdateHello(accountID, state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (l *ProfitLimitObserver) UpdateMaximumFromTotals(accountID int64, profitMax float64) (*dbRedis.AccountProfitLimitState, error) {
	return l.dsProfit.Update(accountID, func(state *dbRedis.AccountProfitLimitState) error {
		state.Maximum = profitMax
		return nil
	})
}

func (l *ProfitLimitObserver) UpdateAndPublish(option *pb.Option) {
	var startDecrease bool
	state, err := l.dsProfit.Update(option.Account.Id, func(state *dbRedis.AccountProfitLimitState) error {
		state.IncreaseCurrent(option.Revenue - option.Bet)
		if state.Current > 0 && !state.IsDecreasing {
			startDecrease = true
			state.IsDecreasing = true
			state.TimeOfReset = time.Now().UTC().Add(l.cfg.ProfitConfig.ResetLimitTime).UnixNano()
			return nil
		}
		return fmt.Errorf("IGNORE DEC for [%d] with [%f]", state.AccountID, state.Current)
	})
	if err != nil {
		l.logger.Debug(err)
		return
	}
	if startDecrease {
		l.logger.Debugf("START DEC for [%d] with [%f]", state.AccountID, state.Current)
		time.AfterFunc(l.cfg.ProfitConfig.ResetLimitTime, func() {
			err := l.onFinishDecrease(state.AccountID, false)
			if err != nil {
				l.logger.Error(err)
			}
		})
		err = l.publishAccountProfitLimitUpdateBegin(state.AccountID, state)
		if err != nil {
			l.logger.Error(err)
		}
	}
}

func (l *ProfitLimitObserver) onFinishDecrease(accountID int64, IsFullReset bool) error {
	var profitChanged bool
	state, err := l.dsProfit.Update(accountID, func(state *dbRedis.AccountProfitLimitState) error {
		l.logger.Debugf("handleDelayedProfit BEFORE [%d - %f]", accountID, state.Current)
		beforeCurrent := state.Current
		if IsFullReset {
			state.Current = 0
		} else {
			state.IncreaseCurrent(-l.cfg.ProfitConfig.ResetLimitValue)
			state.TimeOfReset = 0
			state.IsDecreasing = false
		}
		if state.Current > 0 && !state.IsDecreasing {
			state.IsDecreasing = true
			state.TimeOfReset = time.Now().UTC().Add(l.cfg.ProfitConfig.ResetLimitTime).UnixNano()
		}
		if beforeCurrent != state.Current {
			profitChanged = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if profitChanged {
		l.logger.Debugf("handleDelayedProfit AFTER [%d - %f]", accountID, state.Current)
		isOffline, err := l.notifyOfflinePushNotice(accountID)
		if err != nil {
			return err
		}
		if !isOffline {
			err = l.publishAccountProfitLimitUpdateEnd(accountID, state)
			if err != nil {
				return err
			}
		}
	}
	if state.IsDecreasing {
		l.logger.Debugf("START DEC for [%d] with [%f]", state.AccountID, state.Current)
		time.AfterFunc(l.cfg.ProfitConfig.ResetLimitTime, func() {
			err := l.onFinishDecrease(state.AccountID, false)
			if err != nil {
				l.logger.Error(err)
			}
		})
		err = l.publishAccountProfitLimitUpdateBegin(state.AccountID, state)
		if err != nil {
			return err
		}
	}
	return nil
}

// Publishers

func (l *ProfitLimitObserver) notifyOfflinePushNotice(accountID int64) (bool, error) {
	isOnline, err := l.dsRuntime.IsOnline(accountID)
	if err != nil {
		return false, err
	}
	if isOnline {
		l.logger.Debugf("Account [%d] is online", accountID)
		return false, nil
	}
	pushPayload := &pb.PushPayload{
		Type:      pb.PushPayloadTypeSingle,
		AccountID: accountID,
	}
	data, err := pushPayload.Marshal()
	if err != nil {
		return false, err
	}
	push := &pb.Push{
		Type:    pb.PushTemplateProfitLimitDecrease,
		Payload: data,
	}
	l.logger.Debugf("[PUSH notify] PushTemplateProfitLimitDecrease for Account [%d]", accountID)
	return true, l.commander.EventToService(queue.EventKey(queue.NotificatorPushSendKey), pb.EventTypePushSend, push)
}

func (l *ProfitLimitObserver) publishAccountProfitLimitUpdateHello(accountID int64, activity *dbRedis.AccountProfitLimitState) error {
	return l.publishAccountProfitLimitUpdate(pb.AccountProfitLimitTypeHello, accountID,
		activity.Current, activity.Maximum, l.cfg.ProfitConfig.ResetLimitValue, activity.TimeOfReset)
}

func (l *ProfitLimitObserver) publishAccountProfitLimitUpdateLight(accountID int64, activity *dbRedis.AccountProfitLimitState) error {
	return l.publishAccountProfitLimitUpdate(pb.AccountProfitLimitTypeLight, accountID,
		activity.Current, activity.Maximum, 0, 0)
}

func (l *ProfitLimitObserver) publishAccountProfitLimitUpdateBegin(accountID int64, activity *dbRedis.AccountProfitLimitState) error {
	return l.publishAccountProfitLimitUpdate(pb.AccountProfitLimitTypeBegin, accountID,
		activity.Current, activity.Maximum, 0, activity.TimeOfReset)
}

func (l *ProfitLimitObserver) publishAccountProfitLimitUpdateEnd(accountID int64, activity *dbRedis.AccountProfitLimitState) error {
	return l.publishAccountProfitLimitUpdate(pb.AccountProfitLimitTypeEnd, accountID,
		activity.Current, activity.Maximum, 0, 0)
}

func (l *ProfitLimitObserver) publishAccountProfitLimitUpdate(msgType pb.AccountProfitLimitType, accountID int64, currentProfit float64, maxProfit float64, deltaProfit float64, timeOfResetNS int64) error {
	return l.commander.ToAccount(accountID, pb.EventTypeAccountProfit,
		&pb.AccountProfitLimit{
			Type:               msgType,
			AccountID:          accountID,
			TimeOfReset:        timeOfResetNS,
			MaxProfitLimit:     maxProfit,
			DeltaProfitLimit:   deltaProfit,
			CurrentProfitLimit: currentProfit,
		})
}
