package award

import (
	"sync"

	"github.com/gogo/protobuf/proto"

	"gitlab.binatex.com/platform/hydra/pb"
	"gitlab.binatex.com/platform/hydra/util/queue"
)

func NewRouter(m *Mediator) (out *Router, err error) {
	profit, err := NewProfitLimiter(m)
	if err != nil {
		return nil, err
	}
	total, err := NewTotalEffectObserver(m)
	if err != nil {
		return nil, err
	}
	sequence, err := NewOptionSequenceAwardWatcher(m)
	out = &Router{
		Mediator: m,
		total:    total,
		profit:   profit,
		sequence: sequence,
	}
	err = out.initCommander()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Router is an entry point for RabbitMQ subscribers
type Router struct {
	*Mediator

	total    *TotalEffectObserver
	profit   *ProfitLimitObserver
	sequence *OptionSequenceAwardWatcher
}

func (r *Router) Start() error {
	err := r.total.Start()
	if err != nil {
		return err
	}
	err = r.profit.Start()
	if err != nil {
		return err
	}
	for {
		select {
		case hello := <-r.chanHello:
			go r.onHello(hello.Account)
		case option := <-r.chanOptionClose:
			go r.onOptionClose(option)
		case effects := <-r.chanTotal:
			go r.onTotal(effects)
		}
	}
}

func (r *Router) onHello(accountID int64) {
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()

		_, err := r.sequence.InitAccount(accountID)
		if err != nil {
			r.logger.Error(err)
		}
	}()
	go func() {
		wg.Add(1)
		defer wg.Done()

		totalState, err := r.total.InitAccount(accountID)
		if err != nil {
			r.logger.Error(err)
			return
		}
		// Depend on 'Total'
		_, err = r.profit.InitAccount(accountID, totalState.GetProfitMax())
		if err != nil {
			r.logger.Error(err)
			return
		}
	}()
	wg.Wait()
}

func (r *Router) onOptionClose(option *pb.Option) {
	r.profit.UpdateAndPublish(option)
	err := r.sequence.UpdateAndPublish(option)
	if err != nil {
		r.logger.Error(err)
	}
}

func (r *Router) onTotal(effects []*pb.AccountBuff) {
	accountID, totalState, err := r.total.UpdateAndPublish(effects)
	if err != nil {
		r.logger.Error(err)
		return
	}
	for _, state := range totalState.Totals {
		switch state.BuffType {
		case pb.BuffTypeSteppedMaxProfitPerDayLimit:
			profitState, err := r.profit.UpdateMaximumFromTotals(accountID, totalState.GetProfitMax())
			if err != nil {
				r.logger.Error(err)
				return
			}
			err = r.profit.publishAccountProfitLimitUpdateLight(accountID, profitState)
			if err != nil {
				r.logger.Errorf("publishAccountProfitLimitUpdateLight [%s]", err.Error())
			}
		case pb.BuffTypeSteppedCashbackPerDayLimit:
			err = r.commander.ObjectToService(queue.EventKey(queue.MoneyboxBuffUpdateKey), &pb.Hello{Account: accountID})
			if err != nil {
				r.logger.Errorf("publishMoneyboxUpdate per day [%s]", err.Error())
			}
		case pb.BuffTypeSteppedCashbackPercentIncrease:
			err = r.commander.ObjectToService(queue.EventKey(queue.MoneyboxBuffUpdateKey), &pb.Hello{Account: accountID})
			if err != nil {
				r.logger.Errorf("publishMoneyboxUpdate percent [%s]", err.Error())
			}
		}
	}
}

// INNER LOGIC

func (r *Router) initCommander() (err error) {
	r.commander, err = r.commander.
		WithPubIN().WithPubOUT().
		WithSubOptionClosed(func(msg proto.Message, event queue.Event) error {
			option, ok := msg.(*pb.Option)
			if !ok {
				return ErrBadCast
			}
			r.chanOptionClose <- option
			return nil
		}).
		Build()
	if err != nil {
		return err
	}
	return nil
}
