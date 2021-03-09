package award

import (
	"fmt"

	"github.com/jackc/pgx"

	"gitlab.binatex.com/platform/hydra/models/db"
	"gitlab.binatex.com/platform/hydra/models/dbRedis"
	"gitlab.binatex.com/platform/hydra/pb"
)

const maxProfitTradeGroupName = "USD"

type TradeGroupStats struct {
	BetMin    float64
	BetMax    float64
	ProfitMax float64
	OptionMax int32
}

func NewTotalEffectObserver(mediator *Mediator) (*TotalEffectObserver, error) {
	dsTotal, err := NewTotalState(mediator.ctx, mediator.logger, mediator.reCli)
	if err != nil {
		return nil, err
	}
	out := &TotalEffectObserver{
		Mediator: mediator,
		dsTotal:  dsTotal,
	}
	return out, nil
}

// TotalEffectObserver looks for changing state of user && recalculate it to more comport struct for back'n'front
type TotalEffectObserver struct {
	*Mediator

	dsTotal *TotalState

}

func (o *TotalEffectObserver) Start() error {
	atc := new(db.TradeGroup)
	err := atc.FindByName(maxProfitTradeGroupName)
	if err != nil {
		return err
	}
	return o.dsTotal.Start(
		o.cfg.EffectConfig.AccumTypes,
		atc.MinBet.Float,
		atc.MaxBet.Float,
		atc.MaxProfit.Float,
		atc.MaxOptions.Int,
	)
}

func (o *TotalEffectObserver) InitAccount(accountID int64) (*dbRedis.AccountTotalState, error) {
	accountEffects := new(db.AccountBuffList)
	err := accountEffects.FindActiveByTypesAndAccountID(accountID, o.cfg.EffectConfig.AccumTypes...)
	if err != nil {
		if err == pgx.ErrNoRows {
			o.logger.Errorf("Total::Init err for Account [%d]: effects was NOT created yet", accountID)
		}
		return nil, err
	}
	state := o.dsTotal.InitAccountState(accountID, accountEffects.ToProto())
	err = o.commander.ToAccount(accountID, pb.EventTypeAccountBuffTotalState,
		&pb.AccountBuffTotalStateList{States: state.Totals})
	if err != nil {
		return nil, err
	}
	o.logger.Debugf("Total::Init success for Account [%d]", accountID)
	return state, nil
}

func (o *TotalEffectObserver) UpdateAndPublish(effects []*pb.AccountBuff) (int64, *dbRedis.AccountTotalState, error) {
	if len(effects) == 0 {
		return 0, nil, fmt.Errorf("TotalEffectObserver::Update err: effects are empty")
	}
	accountID := effects[0].Account.Id
	state, err := o.dsTotal.Update(accountID, effects)
	if err != nil {
		return 0, nil, fmt.Errorf("TotalEffectObserver::Update err for Account [%d]: %s", accountID, err)
	}
	err = o.commander.ToAccount(accountID, pb.EventTypeAccountBuffTotalState,
		&pb.AccountBuffTotalStateList{States: state.Totals})
	if err != nil {
		o.logger.Errorf("TotalEffectObserver::Update err for Account [%d]: %s", accountID, err)
	} else {
		o.logger.Debugf("Total::Update success for Account [%d]", accountID)
	}
	return accountID, state, nil
}
