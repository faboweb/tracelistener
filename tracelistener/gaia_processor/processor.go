package gaia_processor

import (
	"fmt"

	"github.com/allinbits/demeris-backend/models"

	"github.com/allinbits/demeris-backend/tracelistener"
	"github.com/allinbits/demeris-backend/tracelistener/config"
	"github.com/cosmos/cosmos-sdk/codec"
	gaia "github.com/cosmos/gaia/v4/app"
	"go.uber.org/zap"
)

type Module interface {
	FlushCache() []tracelistener.WritebackOp
	OwnsKey(key []byte) bool
	Process(data tracelistener.TraceOperation) error
	ModuleName() string
	TableSchema() string
}

// TODO: this singleton MUST go away.
var p Processor

type Processor struct {
	l                *zap.SugaredLogger
	writeChan        chan tracelistener.TraceOperation
	writebackChan    chan []tracelistener.WritebackOp
	cdc              codec.Marshaler
	migrations       []string
	lastHeight       uint64
	chainName        string
	moduleProcessors []Module
}

func (p *Processor) OpsChan() chan tracelistener.TraceOperation {
	return p.writeChan
}

func (p *Processor) WritebackChan() chan []tracelistener.WritebackOp {
	return p.writebackChan
}

func (p *Processor) DatabaseMigrations() []string {
	return p.migrations
}

func (p *Processor) ErrorsChan() chan error {
	return p.errorsChan
}

func New(logger *zap.SugaredLogger, cfg *config.Config) (tracelistener.DataProcessor, error) {
	c := cfg.Gaia

	if c.ProcessorsEnabled == nil {
		c.ProcessorsEnabled = []string{"bank", "delegations", "auth"}
	}

	var mp []Module
	var tableSchemas []string

	for _, ep := range c.ProcessorsEnabled {
		p, err := processorByName(ep, logger)
		if err != nil {
			return nil, err
		}

		mp = append(mp, p)
		tableSchemas = append(tableSchemas, p.TableSchema())
	}

	logger.Infow("gaia Processor initialized", "processors", c.ProcessorsEnabled)

	p = Processor{
		chainName:        cfg.ChainName,
		l:                logger,
		writeChan:        make(chan tracelistener.TraceOperation),
		writebackChan:    make(chan []tracelistener.WritebackOp),
		moduleProcessors: mp,
		migrations:       tableSchemas,
	}

	cdc, _ := gaia.MakeCodecs()
	p.cdc = cdc

	go p.lifecycle()

	return &p, nil
}

func (p *Processor) AddModule(m Module) error {
	mn := m.ModuleName()
	for _, em := range p.moduleProcessors {
		if em.ModuleName() == mn {
			return fmt.Errorf("cannot add module %s more than one time", mn)
		}
	}

	p.moduleProcessors = append(p.moduleProcessors, m)

	return nil
}

func processorByName(name string, logger *zap.SugaredLogger) (Module, error) {
	switch name {
	default:
		return nil, fmt.Errorf("unkonwn Processor %s", name)
	case (&bankProcessor{}).ModuleName():
		return &bankProcessor{heightCache: map[bankCacheEntry]models.BalanceRow{}, l: logger}, nil
	case (&ibcConnectionsProcessor{}).ModuleName():
		return &ibcConnectionsProcessor{connectionsCache: map[connectionCacheEntry]models.IBCConnectionRow{}, l: logger}, nil
	case (&liquidityPoolProcessor{}).ModuleName():
		return &liquidityPoolProcessor{poolsCache: map[uint64]models.PoolRow{}, l: logger}, nil
	case (&liquiditySwapsProcessor{}).ModuleName():
		return &liquiditySwapsProcessor{swapsCache: map[uint64]models.SwapRow{}, l: logger}, nil
	case (&delegationsProcessor{}).ModuleName():
		return &delegationsProcessor{
			insertHeightCache: map[delegationCacheEntry]models.DelegationRow{},
			deleteHeightCache: map[delegationCacheEntry]models.DelegationRow{},
			l:                 logger,
		}, nil
	case (&ibcDenomTracesProcessor{}).ModuleName():
		return &ibcDenomTracesProcessor{
			l:                logger,
			denomTracesCache: map[string]models.IBCDenomTraceRow{},
		}, nil
	case (&ibcChannelsProcessor{}).ModuleName():
		return &ibcChannelsProcessor{channelsCache: map[channelCacheEntry]models.IBCChannelRow{}, l: logger}, nil
	case (&authProcessor{}).ModuleName():
		return &authProcessor{
			l:           logger,
			heightCache: map[authCacheEntry]models.AuthRow{},
		}, nil
	}
}

func (p *Processor) lifecycle() {
	for data := range p.writeChan {
		if data.BlockHeight != p.lastHeight && data.BlockHeight != 0 {
			wb := make([]tracelistener.WritebackOp, 0, len(p.moduleProcessors))

			for _, mp := range p.moduleProcessors {
				cd := mp.FlushCache()
				for _, entry := range cd {
					if entry.Data == nil {
						continue
					}

					for i := 0; i < len(entry.Data); i++ {
						entry.Data[i] = entry.Data[i].WithChainName(p.chainName)
					}
					wb = append(wb, entry)
				}
			}

			p.writebackChan <- wb

			p.l.Infow("processed new block", "height", p.lastHeight)

			p.lastHeight = data.BlockHeight
		}

		for _, mp := range p.moduleProcessors {
			if !mp.OwnsKey(data.Key) {
				continue
			}

			if err := mp.Process(data); err != nil {
				p.l.Errorw(
					"error while processing data",
					"error", err,
					"data", data,
					"moduleName", mp.ModuleName())
			}
		}
	}
}
