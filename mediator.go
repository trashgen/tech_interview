package award

import (
	"context"
	"fmt"
	"gitlab.binatex.com/platform/hydra/models/dbRedis"
	"gitlab.binatex.com/platform/hydra/pb"
	"gitlab.binatex.com/platform/hydra/util/queue"

	"github.com/sirupsen/logrus"

	"gitlab.binatex.com/platform/hydra/util/redis"
)

const chanCapacity = 1000

var ErrBadCast = fmt.Errorf("invalid cast types")

func NewMediator(ctx context.Context, logger *logrus.Entry, cfg *Config, reCli *redis.Client) (out *Mediator, err error) {
	out = &Mediator{
		ctx:                 ctx,
		logger:              logger,
		cfg:                 cfg,
		reCli:               reCli,
		commander:           queue.NewCommander(ctx, logger),
		dsRuntime:           dbRedis.NewRuntimeState(ctx, logger, reCli),
		chanHello:           make(chan *pb.Hello, chanCapacity),
		chanTotal:           make(chan []*pb.AccountBuff, chanCapacity),
		chanOptionClose:     make(chan *pb.Option, chanCapacity),
		awardRatingLink:     NewStaticAwardRatingLink(logger),
		awardCollectionLink: NewStaticAwardCollectionLink(logger),
		awardItemBoughtLink: NewStaticAwardItemBoughtLink(logger),
	}
	out.dsActivity, err = dbRedis.NewActivityStateMaster(out.ctx, out.logger, cfg.EffectConfig.AccumTypes, reCli)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Mediator is 'mediator' pattern with shared data && sources in microservice
type Mediator struct {
	ctx    context.Context
	logger *logrus.Entry
	cfg    *Config
	reCli  *redis.Client

	commander  *queue.Commander
	dsRuntime  *dbRedis.RuntimeState
	dsActivity *dbRedis.ActivityStateMaster

	// Event-based awards
	awardRatingLink     *StaticAwardRatingLink
	awardCollectionLink *StaticAwardCollectionLink
	awardItemBoughtLink *StaticAwardItemBoughtLink

	// Chans for decrease Rabbit loading
	chanHello       chan *pb.Hello
	chanTotal       chan []*pb.AccountBuff
	chanOptionClose chan *pb.Option
}
