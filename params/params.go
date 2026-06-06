package params

import "math/big"

const (
	CoinName       = "NEVEL"
	Symbol         = "NEVEL"
	SmallestUnit   = "neveloshi"
	UnitsPerNEVEL  = uint64(100_000_000)
	MaxSupply      = uint64(21_000_000_000) * UnitsPerNEVEL
	TargetBlockSec = int64(60)
	InitialReward  = uint64(500) * UnitsPerNEVEL

	HalvingInterval              = uint64(2_102_400)
	DifficultyAdjustmentInterval = uint64(720)
	DifficultyWindow             = DifficultyAdjustmentInterval
	ExpectedRetargetSec          = TargetBlockSec * int64(DifficultyAdjustmentInterval)
	CoinbaseMaturity             = uint64(100)
	MaxBlockBytes                = int64(2_000_000)
	DefaultMempoolMaxByte        = int64(64 * 1024 * 1024)
	MinRelayFeePerByte           = uint64(1)

	ProductionPowBits = uint32(0x1d00ffff)
	RegtestPowBits    = uint32(0x207fffff)
)

const (
	MainnetID = "nevel-mainnet-1"
	TestnetID = "nevel-testnet-1"
	RegtestID = "nevel-regtest-1"
)

type Network struct {
	ID            string
	AddressPrefix string
	DefaultRPC    string
	GenesisBits   uint32
	InitialBits   uint32
	PowLimit      *big.Int
}

func MaxPowLimit() *big.Int {
	return new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
}

func Mainnet() Network {
	return Network{
		ID:            MainnetID,
		AddressPrefix: "nevel",
		DefaultRPC:    "127.0.0.1:8332",
		GenesisBits:   ProductionPowBits,
		InitialBits:   ProductionPowBits,
		PowLimit:      CompactToBig(ProductionPowBits),
	}
}
func Testnet() Network {
	return Network{
		ID:            TestnetID,
		AddressPrefix: "tnevel",
		DefaultRPC:    "127.0.0.1:18332",
		GenesisBits:   ProductionPowBits,
		InitialBits:   ProductionPowBits,
		PowLimit:      CompactToBig(ProductionPowBits),
	}
}
func Regtest() Network {
	return Network{
		ID:            RegtestID,
		AddressPrefix: "rnevel",
		DefaultRPC:    "127.0.0.1:18443",
		GenesisBits:   RegtestPowBits,
		InitialBits:   RegtestPowBits,
		PowLimit:      CompactToBig(RegtestPowBits),
	}
}

func ByName(name string) Network {
	switch name {
	case "mainnet":
		return Mainnet()
	case "testnet":
		return Testnet()
	default:
		return Regtest()
	}
}

func CompactToBig(compact uint32) *big.Int {
	size := compact >> 24
	word := compact & 0x007fffff
	bn := new(big.Int).SetUint64(uint64(word))
	if size <= 3 {
		bn.Rsh(bn, uint(8*(3-size)))
	} else {
		bn.Lsh(bn, uint(8*(size-3)))
	}
	if compact&0x00800000 != 0 {
		bn.Neg(bn)
	}
	return bn
}

func BigToCompact(n *big.Int) uint32 {
	if n.Sign() == 0 {
		return 0
	}
	bytes := n.Bytes()
	size := len(bytes)
	var compact uint32
	if size <= 3 {
		compact = uint32(new(big.Int).Lsh(new(big.Int).Set(n), uint(8*(3-size))).Uint64())
	} else {
		compact = uint32(new(big.Int).Rsh(new(big.Int).Set(n), uint(8*(size-3))).Uint64())
	}
	if compact&0x00800000 != 0 {
		compact >>= 8
		size++
	}
	compact |= uint32(size) << 24
	return compact
}
