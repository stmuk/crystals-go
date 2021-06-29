package kyber

import (
	"crypto/rand"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/sha3"
)

func (k *Kyber) CPAKeyGen() ([]byte, []byte) {
	seed := make([]byte, SEEDBYTES)
	rand.Read(seed)

	K := k.params.K
	ETA1 := k.params.ETA1

	var rho, sseed [SEEDBYTES]byte
	state := sha3.New512()
	state.Write(seed)
	hash := state.Sum(nil)
	copy(rho[:], hash[:32])
	copy(sseed[:], hash[32:])

	Ahat := expandSeed(rho[:], false, K)

	shat := make(Vec, K)
	for i := 0; i < K; i++ {
		shat[i] = polyGetNoise(ETA1, sseed[:], byte(i))
		shat[i].ntt()
		shat[i].reduce()
	}

	ehat := make(Vec, K)
	for i := 0; i < K; i++ {
		ehat[i] = polyGetNoise(ETA1, sseed[:], byte(i+K))
		ehat[i].ntt()
	}

	t := make(Vec, K)
	for i := 0; i < K; i++ {
		t[i] = vecPointWise(Ahat[i], shat, K)
		t[i].toMont()
		t[i] = add(t[i], ehat[i])
		t[i].reduce()
	}

	return k.PackPK(&PublicKey{T: t, Rho: rho[:]}), k.PackPKESK(&PKEPrivateKey{S: shat})
}

func (k *Kyber) CPAEncaps(ppk []byte) ([]byte, []byte) {
	var msg [32]byte
	rand.Read(msg[:])
	kr := sha3.Sum512(msg[:])

	K := k.params.K
	pk := k.UnpackPK(ppk)
	Ahat := expandSeed(pk.Rho[:], true, K)

	sp := make(Vec, K)
	for i := 0; i < K; i++ {
		sp[i] = polyGetNoise(k.params.ETA1, kr[32:], byte(i))
		sp[i].ntt()
		sp[i].reduce()
	}
	ep := make(Vec, K)
	for i := 0; i < K; i++ {
		ep[i] = polyGetNoise(eta2, kr[32:], byte(i+K))
		ep[i].ntt()
	}
	epp := polyGetNoise(eta2, kr[32:], byte(2*K))
	epp.ntt()

	u := make(Vec, K)
	for i := 0; i < K; i++ {
		u[i] = vecPointWise(Ahat[i], sp, K)
		u[i].toMont()
		u[i] = add(u[i], ep[i])
		u[i].invntt()
		u[i].reduce()
		u[i].fromMont()
	}

	v := vecPointWise(pk.T, sp, K)
	v.toMont()
	v = add(v, epp)
	m := polyFromMsg(kr[:32])
	m.ntt()
	v = add(v, m)
	v.invntt()
	v.reduce()
	v.fromMont()

	c := make([]byte, k.params.SIZEC)
	copy(c[:], u.compress(k.params.DU, K))
	copy(c[K*k.params.DU*n/8:], v.compress(k.params.DV))

	ss := blake2s.Sum256(kr[:32])
	return c, ss[:]
}

func (k *Kyber) CPADecaps(psk, c []byte) []byte {
	sk := k.UnpackPKESK(psk)
	K := k.params.K
	uhat := decompressVec(c[:K*k.params.DU*n/8], k.params.DU, K)
	uhat.ntt(K)
	v := decompressPoly(c[K*k.params.DU*n/8:], k.params.DV)
	v.ntt()

	m := vecPointWise(sk.S, uhat, K)
	m.toMont()
	m = sub(v, m)
	m.invntt()
	m.reduce()
	m.fromMont()
	kr := polyToMsg(m)
	ss := blake2s.Sum256(kr)
	return ss[:]
}
