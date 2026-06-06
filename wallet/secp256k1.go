package wallet

import (
	"crypto/elliptic"
	"math/big"
)

type secp256k1Curve struct{ *elliptic.CurveParams }

var secp256k1 elliptic.Curve = initSecp256k1()

func initSecp256k1() elliptic.Curve {
	p, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	n, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	gx, _ := new(big.Int).SetString("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 16)
	gy, _ := new(big.Int).SetString("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 16)
	params := &elliptic.CurveParams{Name: "secp256k1"}
	params.P = p
	params.N = n
	params.B = big.NewInt(7)
	params.Gx = gx
	params.Gy = gy
	params.BitSize = 256
	return &secp256k1Curve{CurveParams: params}
}

func (c *secp256k1Curve) IsOnCurve(x, y *big.Int) bool {
	if x == nil || y == nil || x.Sign() < 0 || y.Sign() < 0 || x.Cmp(c.P) >= 0 || y.Cmp(c.P) >= 0 {
		return false
	}
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, c.P)
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Add(x3, c.B)
	x3.Mod(x3, c.P)
	return y2.Cmp(x3) == 0
}

func (c *secp256k1Curve) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	if x1 == nil || y1 == nil {
		return new(big.Int).Set(x2), new(big.Int).Set(y2)
	}
	if x2 == nil || y2 == nil {
		return new(big.Int).Set(x1), new(big.Int).Set(y1)
	}
	p := c.P
	if x1.Cmp(x2) == 0 {
		ysum := new(big.Int).Add(y1, y2)
		ysum.Mod(ysum, p)
		if ysum.Sign() == 0 {
			return nil, nil
		}
		return c.Double(x1, y1)
	}
	num := new(big.Int).Sub(y2, y1)
	den := new(big.Int).Sub(x2, x1)
	den.ModInverse(den.Mod(den, p), p)
	lambda := num.Mul(num, den)
	lambda.Mod(lambda, p)
	x3 := new(big.Int).Mul(lambda, lambda)
	x3.Sub(x3, x1)
	x3.Sub(x3, x2)
	x3.Mod(x3, p)
	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(lambda, y3)
	y3.Sub(y3, y1)
	y3.Mod(y3, p)
	return x3, y3
}

func (c *secp256k1Curve) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	if x1 == nil || y1 == nil || y1.Sign() == 0 {
		return nil, nil
	}
	p := c.P
	num := new(big.Int).Mul(big.NewInt(3), new(big.Int).Mul(x1, x1))
	den := new(big.Int).Mul(big.NewInt(2), y1)
	den.ModInverse(den.Mod(den, p), p)
	lambda := num.Mul(num, den)
	lambda.Mod(lambda, p)
	x3 := new(big.Int).Mul(lambda, lambda)
	x3.Sub(x3, new(big.Int).Mul(big.NewInt(2), x1))
	x3.Mod(x3, p)
	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(lambda, y3)
	y3.Sub(y3, y1)
	y3.Mod(y3, p)
	return x3, y3
}

func (c *secp256k1Curve) ScalarMult(bx, by *big.Int, k []byte) (*big.Int, *big.Int) {
	var x, y *big.Int
	for _, byteVal := range k {
		for bitNum := 7; bitNum >= 0; bitNum-- {
			if x != nil {
				x, y = c.Double(x, y)
			}
			if byteVal&(1<<uint(bitNum)) != 0 {
				x, y = c.Add(x, y, bx, by)
			}
		}
	}
	return x, y
}

func (c *secp256k1Curve) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	return c.ScalarMult(c.Gx, c.Gy, k)
}
