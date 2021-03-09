package award

import (
	"github.com/jackc/pgx"
	"gitlab.binatex.com/platform/hydra/models/db"
	"gitlab.binatex.com/platform/hydra/models/dbRedis"
	"gitlab.binatex.com/platform/hydra/pb"
)

func NewOptionSequenceAwardWatcher(m *Mediator) (*OptionSequenceAwardWatcher, error) {
	dsSequence, err := dbRedis.NewOptionSequenceState(m.ctx, m.logger, m.reCli, m.cfg.GetMaxWinNum())
	if err != nil {
		return nil, err
	}
	out := &OptionSequenceAwardWatcher{
		Mediator:   m,
		dsSequence: dsSequence,
	}
	return out, nil
}

// OptionSequenceAwardWatcher watches and increase counter when 'user action'
type OptionSequenceAwardWatcher struct {
	*Mediator

	dsSequence *dbRedis.OptionSequenceState
}

func (w *OptionSequenceAwardWatcher) InitAccount(accountID int64) (*dbRedis.AccountOptionSequenceState, error) {
	options := new(db.OptionList)
	err := options.FindLastOptionsLimit(accountID, w.cfg.GetMaxWinNum())
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	var currSeqLen int64
	for _, option := range *options {
		if /*condition*/ {
			break
		}
		currSeqLen++
	}
	state := w.dsSequence.InitAccountState(accountID, currSeqLen)
	return state, nil
}

func (w *OptionSequenceAwardWatcher) UpdateAndPublish(option *pb.Option) error {
	isValid, currSeqLen := w.isOptionValid(option)
	if !isValid {
		return nil
	}
	// Logic
}

// INNER LOGIC

func (w *OptionSequenceAwardWatcher) isOptionValid(option *pb.Option) (bool, int64) {
	validOptionStatus := option.Status == pb.OptionStatusClosed || option.Status == pb.OptionStatusEarlyClosed
	if option.State == pb.OptionStateWin && (option.Revenue-option.Bet > 0) && validOptionStatus {
		wasReset, currSeqLen := w.dsSequence.IncrementAndCheck(option.Account.Id)
		if wasReset {
			return false, 0
		}
		return true, currSeqLen
	}
	return false, 0
}
