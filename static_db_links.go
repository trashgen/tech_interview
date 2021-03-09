package award

import (
	"encoding/json"

	"github.com/jackc/pgx"
	"github.com/sirupsen/logrus"

	"gitlab.binatex.com/platform/hydra/models/db"
	"gitlab.binatex.com/platform/hydra/pb"
)

// Some links to static DB data to reorganize it to comport structs

func NewStaticAwardRatingLink(logger *logrus.Entry) *StaticAwardRatingLink {
	return &StaticAwardRatingLink{logger: logger}
}

type StaticAwardRatingLink struct {
	logger *logrus.Entry

	points []int64
	// map[Point]map[EffectID]Weight - struct for util.WeightedRandom()
	effects map[int64]map[int64]int64

	awardRatings []*pb.AwardRating
}

func (l *StaticAwardRatingLink) update() error {
	ars := new(db.AwardRatingList)
	err := ars.All()
	if err != nil {
		if err == pgx.ErrNoRows {
			l.logger.Errorf("StaticAwardRatingLink err: not mapped 'cos no data in DB")
			return nil
		}
		return err
	}
	l.points = make([]int64, 0, ars.Len())
	l.awardRatings = make([]*pb.AwardRating, 0, ars.Len())
	for _, awardRating := range *ars {
		l.points = append(l.points, awardRating.Threshold)
		l.awardRatings = append(l.awardRatings, awardRating.ToProto())
	}
	l.effects = make(map[int64]map[int64]int64)
	for _, rp := range l.points {
		l.effects[rp] = make(map[int64]int64)
	}
	for _, dbRules := range *ars {
		rules := make([]*pb.WeightedBuff, 0)
		err = json.Unmarshal(dbRules.Rules.Bytes, &rules)
		if err != nil {
			l.logger.Errorf("StaticAwardRatingLink err: [%s]", err.Error())
			return err
		}
		for _, rule := range rules {
			l.effects[dbRules.Threshold][rule.BuffID] = rule.Weight
		}
	}
	return nil
}

func NewStaticAwardCollectionLink(logger *logrus.Entry) *StaticAwardCollectionLink {
	return &StaticAwardCollectionLink{logger: logger}
}

type StaticAwardCollectionLink struct {
	logger *logrus.Entry

	points []int64
	// map[CollectionPoint][]BuffID
	effects map[int64][]int64

	awardCollections []*pb.AwardCollection
}

func (l *StaticAwardCollectionLink) update() error {
	ais := new(db.AwardCollectionList)
	err := ais.All()
	if err != nil {
		if err == pgx.ErrNoRows {
			l.logger.Errorf("StaticAwardCollectionLink err: not mapped 'cos no data in DB")
			return nil
		}
		return err
	}
	l.points = make([]int64, 0, ais.Len())
	l.awardCollections = make([]*pb.AwardCollection, 0, ais.Len())
	for _, ac := range *ais {
		l.points = append(l.points, ac.Threshold)
		l.awardCollections = append(l.awardCollections, ac.ToProto())
	}
	l.effects = make(map[int64][]int64)
	for _, point := range l.points {
		l.effects[point] = make([]int64, 0)
	}
	for _, dbRules := range *ais {
		rules := make([]int64, 0)
		err = json.Unmarshal(dbRules.Rules.Bytes, &rules)
		if err != nil {
			l.logger.Errorf("StaticAwardCollectionLink err: [%s]", err.Error())
			return err
		}
		l.effects[dbRules.Threshold] = rules
	}
	return nil
}

func NewStaticAwardItemBoughtLink(logger *logrus.Entry) *StaticAwardItemBoughtLink {
	return &StaticAwardItemBoughtLink{logger: logger}
}

type StaticAwardItemBoughtLink struct {
	logger *logrus.Entry

	points []int64
	// map[Point]map[EffectID]Weight - struct for util.WeightedRandom()
	effects map[int64]map[int64]int64

	awardItemBoughts []*pb.AwardItemBought
}

func (l *StaticAwardItemBoughtLink) update() error {
	ais := new(db.AwardItemList)
	err := ais.All()
	if err != nil {
		if err == pgx.ErrNoRows {
			l.logger.Errorf("StaticAwardItemBoughtLink err: not mapped 'cos no data in DB")
			return nil
		}
		return err
	}
	l.points = make([]int64, 0, ais.Len())
	l.awardItemBoughts = make([]*pb.AwardItemBought, 0, ais.Len())
	for _, awardItem := range *ais {
		l.points = append(l.points, awardItem.Threshold)
		l.awardItemBoughts = append(l.awardItemBoughts, awardItem.ToProto())
	}
	l.effects = make(map[int64]map[int64]int64)
	for _, rp := range l.points {
		l.effects[rp] = make(map[int64]int64)
	}
	for _, dbRules := range *ais {
		rules := make([]*pb.WeightedBuff, 0)
		err = json.Unmarshal(dbRules.Rules.Bytes, &rules)
		if err != nil {
			l.logger.Errorf("StaticAwardItemBoughtLink err: [%s]", err.Error())
			return err
		}
		for _, rule := range rules {
			l.effects[dbRules.Threshold][rule.BuffID] = rule.Weight
		}
	}
	return nil
}
