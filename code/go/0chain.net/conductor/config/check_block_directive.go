package config

import (
	"0chain.net/chaincore/block"
	"encoding/json"
	"errors"
	"github.com/mitchellh/mapstructure"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type (
	CheckBlockDirective struct {
		SharderAddress    string `yaml:"sharder_address" mapstructure:"sharder_address"`
		MinerRankExpected int    `yaml:"miner_rank_expected" mapstructure:"miner_rank_expected"`
	}
)

const (
	checkBlockDirectiveKey = "check_block"
)

func (d *CheckBlockDirective) validate(b *block.Block) error {
	ranks := computeMinersRanks(b.Miners.MapSize(), b.RoundRandomSeed)
	expectedMinerInd := ranks[d.MinerRankExpected]
	expectedMiner := b.MagicBlock.Miners.Nodes[expectedMinerInd]

	if expectedMiner.ID != b.MinerID {
		msg := "unexpected miner id: " + expectedMiner.ID + " expected; " + b.MinerID + " actual"
		log.Println(msg)
		return errors.New(msg)
	}
	log.Println("[OK] miner ID is checked")

	return nil
}

//func ExtractCheckBlockDirective(directives []Directive) (*CheckBlockDirective, error) {
//	for _, dir := range directives {
//		name, dirMap, ok := dir.unwrap()
//		if !ok {
//			return nil, errors.New("invalid directives")
//		}
//		if name == checkBlockDirectiveKey {
//			cbDir := new(CheckBlockDirective)
//			if err := cbDir.Decode(dirMap); err != nil {
//				return nil, err
//			}
//			return cbDir, nil
//		}
//	}
//
//	return nil, errors.New("directive not found")
//} TODO

func (d *CheckBlockDirective) Decode(iface interface{}) error {
	return mapstructure.Decode(iface, d)
}

func checkBlock(name string, _ Executor, val interface{}, _ time.Duration) (err error) {
	if name != checkBlockDirectiveKey {
		return errors.New("name is invalid")
	}

	dir := new(CheckBlockDirective)
	if err := dir.Decode(val); err != nil {
		return err
	}

	b, err := requestBlockFromSharder(1, dir.SharderAddress)
	if err != nil {
		return err
	}

	mb, err := requestMagicBlockFromSharder(1, dir.SharderAddress)
	if err != nil {
		return err
	}
	b.MagicBlock = mb.MagicBlock

	// computing array for ranks resolving
	for _, miner := range b.Miners.NodesMap {
		mb.Miners.Nodes = append(mb.Miners.Nodes, miner)
	}

	// make check
	return dir.validate(b)
}

type (
	getBlockResp struct {
		Block *block.Block `json:"block"`
	}
)

func requestBlockFromSharder(num int, address string) (*block.Block, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	u.Path = "v1/block/get"
	u.RawQuery = url.Values{
		"round":   []string{strconv.Itoa(num)},
		"content": []string{"full"},
	}.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("requesting block, status not ok: " + resp.Status)
	}

	blob, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	blockResp := new(getBlockResp)
	if err := json.Unmarshal(blob, blockResp); err != nil {
		return nil, err
	}
	return blockResp.Block, nil
}

func requestMagicBlockFromSharder(num int, address string) (*block.Block, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	u.Path = "/v1/block/magic/get"
	u.RawQuery = url.Values{
		"magic_block_number": []string{strconv.Itoa(num)},
	}.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("requesting magic block, status not ok: " + resp.Status)
	}

	blob, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	mb := new(block.Block)
	if err := json.Unmarshal(blob, mb); err != nil {
		return nil, err
	}
	return mb, nil
}

func computeMinersRanks(minersNum int, randomSeed int64) []int {
	return rand.New(rand.NewSource(randomSeed)).Perm(minersNum)
}
