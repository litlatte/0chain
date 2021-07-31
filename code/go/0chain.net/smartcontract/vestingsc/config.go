package vestingsc

import (
	"context"
	"net/url"
	"time"

	zchainErrors "github.com/0chain/gosdk/errors"

	chainstate "0chain.net/chaincore/chain/state"
	configpkg "0chain.net/chaincore/config"
	"0chain.net/chaincore/state"
	"0chain.net/core/common"
)

type config struct {
	MinLock              state.Balance `json:"min_lock"`
	MinDuration          time.Duration `json:"min_duration"`
	MaxDuration          time.Duration `json:"max_duration"`
	MaxDestinations      int           `json:"max_destinations"`
	MaxDescriptionLength int           `json:"max_description_length"`
}

func (c *config) validate() (err error) {
	switch {
	case c.MinLock <= 0:
		return zchainErrors.New("invalid min_lock (<= 0)")
	case toSeconds(c.MinDuration) < 1:
		return zchainErrors.New("invalid min_duration (< 1s)")
	case toSeconds(c.MaxDuration) <= toSeconds(c.MinDuration):
		return zchainErrors.New("invalid max_duration: less or equal to min_duration")
	case c.MaxDestinations < 1:
		return zchainErrors.New("invalid max_destinations (< 1)")
	case c.MaxDescriptionLength < 1:
		return zchainErrors.New("invalid max_description_length (< 1)")
	}
	return
}

//
// helpers
//

// configurations from sc.yaml
func getConfig() (conf *config, err error) {

	const prefix = "smart_contracts.vestingsc."

	conf = new(config)

	// short hand
	var scconf = configpkg.SmartContractConfig
	conf.MinLock = state.Balance(scconf.GetFloat64(prefix+"min_lock") * 1e10)
	conf.MinDuration = scconf.GetDuration(prefix + "min_duration")
	conf.MaxDuration = scconf.GetDuration(prefix + "max_duration")
	conf.MaxDestinations = scconf.GetInt(prefix + "max_destinations")
	conf.MaxDescriptionLength = scconf.GetInt(prefix + "max_description_length")

	err = conf.validate()
	if err != nil {
		return nil, err
	}
	return
}

//
// REST-handler
//

func (vsc *VestingSmartContract) getConfigHandler(context.Context,
	url.Values, chainstate.StateContextI) (interface{}, error) {

	res, err := getConfig()
	if err != nil {
		return nil, common.NewErrInternal(err, "can't get config")
	}
	return res, nil
}
