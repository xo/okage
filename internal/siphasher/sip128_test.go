// Copyright 2014 The Rust Project Developers
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package siphasher

import (
	"encoding/hex"
	"math/bits"
	"testing"
)

// The tests of siphasher src/tests128.rs, and the outputs of siphasher 1.0.2
// for the inputs of the reference vectors of SipHash.

// hasher128 is a hasher of this package, as the Rust traits Hasher and
// Hasher128 give it to the tests.
type hasher128 interface {
	Write(msg []byte)
	Finish128() Hash128
}

// hashWith is hash_with in siphasher src/tests128.rs, for the type Bytes,
// which hashes the bytes of a slice without a length prefix.
func hashWith(st hasher128, x []byte) [16]byte {
	st.Write(x)
	return st.Finish128().AsBytes()
}

// TestSiphash128Idempotent is test_siphash128_idempotent in siphasher
// src/tests128.rs.
func TestSiphash128Idempotent(t *testing.T) {
	hash := func(write func(h *SipHasher)) [16]byte {
		h := NewSipHasher()
		write(&h)
		return h.Finish128().AsBytes()
	}
	val64 := func(h *SipHasher) { h.WriteU64(0xdead_beef_dead_beef) }
	if a, b := hash(val64), hash(val64); a != b {
		t.Error("the hash of a u64 changes")
	}
	val32 := func(h *SipHasher) { h.WriteU32(0xdeadbeef) }
	if a, b := hash(val32), hash(val32); a != b {
		t.Error("the hash of a u32 changes")
	}
}

// TestSiphash128_1_3 is test_siphash128_1_3 in siphasher src/tests128.rs.
func TestSiphash128_1_3(t *testing.T) {
	vecs := [][16]byte{{
		231, 126, 188, 178, 39, 136, 165, 190, 253, 98, 219, 106, 221, 48, 48, 1,
	}}

	var k0 uint64 = 0x_07_06_05_04_03_02_01_00
	var k1 uint64 = 0x_0f_0e_0d_0c_0b_0a_09_08
	var buf []byte
	stateInc := NewSipHasher13WithKeys(k0, k1)

	for tt, vec := range vecs {
		h := NewSipHasher13WithKeys(k0, k1)
		out := hashWith(&h, buf)
		if vec != out {
			t.Errorf("%d: got %x, want %x", tt, out, vec)
		}

		h = NewSipHasher13WithKeys(k0, k1)
		full := hashWith(&h, buf)
		i := stateInc.Finish128().AsBytes()

		if full != i || full != vec {
			t.Errorf("%d: the full hash %x, the incremental hash %x, want %x", tt, full, i, vec)
		}

		buf = append(buf, byte(tt))
		stateInc.Write([]byte{byte(tt)})
	}
}

// TestSiphash128_2_4 is test_siphash128_2_4 in siphasher src/tests128.rs.
func TestSiphash128_2_4(t *testing.T) {
	vecs := [][16]byte{{
		163, 129, 127, 4, 186, 37, 168, 230, 109, 246, 114, 20, 199, 85, 2, 147,
	}}

	var k0 uint64 = 0x_07_06_05_04_03_02_01_00
	var k1 uint64 = 0x_0f_0e_0d_0c_0b_0a_09_08
	var buf []byte
	stateInc := NewSipHasher24WithKeys(k0, k1)

	for tt, vec := range vecs {
		h := NewSipHasher24WithKeys(k0, k1)
		out := hashWith(&h, buf)
		if vec != out {
			t.Errorf("%d: got %x, want %x", tt, out, vec)
		}

		h = NewSipHasher24WithKeys(k0, k1)
		full := hashWith(&h, buf)
		i := stateInc.Finish128().AsBytes()

		if full != i || full != vec {
			t.Errorf("%d: the full hash %x, the incremental hash %x, want %x", tt, full, i, vec)
		}

		buf = append(buf, byte(tt))
		stateInc.Write([]byte{byte(tt)})
	}
}

// TestSiphash128Simple is test_siphash128_simple in siphasher
// src/tests128.rs.
func TestSiphash128Simple(t *testing.T) {
	array := []byte{1, 2, 3}
	key := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	hasher := NewSipHasher13WithKey(key)
	_ = hasher.Hash(array).AsBytes()
	if hasher.Key() != key {
		t.Errorf("the key is %x, want %x", hasher.Key(), key)
	}
}

// TestSiphash128Incremental is test_siphash128_incremental in siphasher
// src/tests128.rs. Upstream only runs the code. The port also compares the
// result with a hash of the whole input.
func TestSiphash128Incremental(t *testing.T) {
	array1 := []byte{1, 2, 3}
	array2 := []byte{4, 5, 6}
	key := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	hasher := NewSipHasher13WithKey(key)
	hasher.Write(array1)
	hasher.Write(array2)
	h := hasher.Finish128().AsBytes()
	if want := NewSipHasher13WithKey(key).Hash([]byte{1, 2, 3, 4, 5, 6}).AsBytes(); h != want {
		t.Errorf("got %x, want %x", h, want)
	}
}

// The outputs of siphasher 1.0.2 with the keys 0x0706050403020100 and
// 0x0f0e0d0c0b0a0908, for the messages of 0 to 63 bytes 0, 1, 2 and so on.
// These are the inputs of the reference vectors of SipHash. A Rust program
// that called SipHasher13::hash and SipHasher24::hash wrote them, as
// Hash128::as_bytes gives them. The first one of each list is the vector of
// the upstream test.
var (
	vectors13 = []string{
		"e77ebcb22788a5befd62db6add303001",
		"fc6f370460d3eda85e0573cc2b2ff063",
		"75787f090569839b855bc9548c6aea95",
		"6bc5ccfa1edcf79f4823187712ebd743",
		"0c784e71ac2b285a9f8e92e78fbf2c25",
		"f328db89345b620c795229a42695843e",
		"dcd03d29f743e7100951b0e83985a6f8",
		"1084b923f2aae0c3a62f2ec80848ab77",
		"aa12fee1d5e3dab4724f16ab35f9c799",
		"81ddb8042cf33994f4720e0094137c42",
		"4faa541d5d498e89ba0ea4c387b22fb4",
		"723b9af3554491dbb1d6633dfc6e0c4e",
		"e53f92859e4819a8dc0695739fea8c65",
		"b2f858c7c9ea801d53d603596d657844",
		"87e76268dbc9227226b0ca665f64e378",
		"c17e5505b2bd526c2921cdec1e7e0109",
		"d0a8d95715518eebb513b0f83d9e1793",
		"234126f93fbb668d975112e8febdf7ec",
		"ef42f03db78f704d023c449f16b7092b",
		"abf76238c20af161b2314b4d5526bce9",
		"3c2c2f11bb90cf0be335ca9b2e91e9b7",
		"2a7a680f22a02a92f45149d20fece0ef",
		"c9a8d130231dd43e42e6456957f83779",
		"1d127b84405ceab99fd8775a9be6c559",
		"9e4bf837bcfd92cace09d2061a84d04a",
		"39031a965d73b4af5a274d18f973b1d2",
		"7f4d0a1209d67e4ed06f7538e1cfad64",
		"e61ee240fbdcce38969f4cd24927dd93",
		"4c3ba2b37b0fdd8cfa5e95c189b29414",
		"e06fd4ca066fecdd54068a5ad8896f86",
		"5ca84c34139c6580a88af24990720706",
		"42ea961c5b3c858b17c3e550dfa79010",
		"406c44dee67857b2943160f30c7417d3",
		"c5f57bae1320fcf4b4e868e71d56c66b",
		"04bf737a5b676be7c3de05017df4bff9",
		"5163c9c03f1907ea1044ed5c30727b4f",
		"37a110f002718edad24b3f9ee453f140",
		"b9877e381aedd3da08c33e75ff23ac10",
		"7c5004005ec5da4c5ac9440e5c723193",
		"81b8243783dbc646ca9d0cd82abdb46c",
		"505720543eb9b413d50b3cfad9eef938",
		"945f594de72411e4d335be874456d8f3",
		"37923b3e371777b21170bf9d7e62f602",
		"3ad4e7c85764964611eb0a6c4d62de56",
		"cd91396c44af4f5185578d9dd9803f0a",
		"fe28158e727b868f3903c9acda64a258",
		"40cc10b8288ce5f0bc3ac0b68a0eebc8",
		"6f1490f540699a3cd4974420ecc92737",
		"d505f1b75e1a84a603c43583b2ed0308",
		"491573cfd72bb4682b7ca5880e1c8d6f",
		"3ed69cfe45ab403f2fd2ad959ba27666",
		"8be839ef1b20b57c83ba7eb6a8c22b6a",
		"1409186ab42231fedee18162cf1cb4ca",
		"2bf3ccc24ab672cf151fb8d2f3f3069b",
		"b9b93a2882d6025cdb8c56fa13f7537b",
		"d97cca3694fb206db8bd1f3650c33322",
		"94ec2e19a40be41af3940d6b30c49384",
		"4b41603f209a045be140a341a3dffe10",
		"23fbcb309f1cf094890755ab1b426569",
		"e7d9b65690918a2b232f2f5c12c8300e",
		"ade83cf7e7f3847b36fa4b54b00dce61",
		"0610c5f2ee571c8ac80cbfe538bdf1c7",
		"271d5d00fbdb5d155d9dcea97cb40218",
		"4c5800e34efe426f079f6b0aa75260ad",
	}
	vectors24 = []string{
		"a3817f04ba25a8e66df67214c7550293",
		"da87c1d86b99af44347659119b22fc45",
		"8177228da4a45dc7fca38bdef60affe4",
		"9c70b60c5267a94e5f33b6b02985ed51",
		"f88164c12d9c8faf7d0f6e7c7bcd5579",
		"1368875980776f8854527a07690e9627",
		"14eeca338b208613485ea0308fd7a15e",
		"a1f1ebbed8dbc153c0b84aa61ff08239",
		"3b62a9ba6258f5610f83e264f31497b4",
		"264499060ad9baabc47f8b02bb6d71ed",
		"00110dc378146956c95447d3f3d0fbba",
		"0151c568386b6677a2b4dc6f81e5dc18",
		"d626b266905ef35882634df68532c125",
		"9869e247e9c08b10d029934fc4b952f7",
		"31fcefac66d7de9c7ec7485fe4494902",
		"5493e99933b0a8117e08ec0f97cfc3d9",
		"6ee2a4ca67b054bbfd3315bf85230577",
		"473d06e8738db89854c066c47ae47740",
		"a426e5e423bf4885294da481feaef723",
		"78017731cf65fab074d5208952512eb1",
		"9e25fc833f2290733e9344a5e83839eb",
		"568e495abe525a218a2214cd3e071d12",
		"4a29b54552d16b9a469c10528eff0aae",
		"c9d184ddd5a9f5e0cf8ce29a9abf691c",
		"2db479ae78bd50d8882a8a178a6132ad",
		"8ece5f042d5e447b5051b9eacb8d8f6f",
		"9c0b53b4b3c307e87eaee08678141f66",
		"abf248af69a6eae4bfd3eb2f129eeb94",
		"0664da1668574b88b935f3027358aef4",
		"aa4b9dc4bf337de90cd4fd3c467c6ab7",
		"ea5c7f471faf6bde2b1ad7d4686d2287",
		"2939b0183223fafc1723de4f52c43d35",
		"7c3956ca5eeafc3e363e9d556546eb68",
		"77c6077146f01c32b6b69d5f4ea9ffcf",
		"37a6986cb8847edf0925f0f1309b54de",
		"a705f0e69da9a8f907241a2e923c8cc8",
		"3dc47d1f29c448461e9e76ed904f6711",
		"0d62bf01e6fc0e1a0d3c4751c5d3692b",
		"8c03468bca7c669ee4fd5e084bbee7b5",
		"528a5bb93baf2c9c4473cce5d0d22bd9",
		"df6a301e95c95dad97ae0cc8c6913bd8",
		"801189902c857f39e73591285e70b6db",
		"e617346ac9c231bb3650ae34ccca0c5b",
		"27d93437efb721aa401821dcec5adf89",
		"89237d9ded9c5e78d8b1c9b166cc7342",
		"4a6d8091bf5e7d651189fa94a250b14c",
		"0e33f96055e7ae893ffc0e3dcf492902",
		"e61c432b720b19d18ec8d84bdc63151b",
		"f7e5aef549f782cf379055a608269b16",
		"438d030fd0b7a54fa837f2ad201a6403",
		"a590d3ee4fbf04e3247e0d27f286423f",
		"5fe2c1a172fe93c4b15cd37caef9f538",
		"2c97325cbd06b36eb2133dd08b3a017c",
		"92c814227a6bca949ff0659f002ad39e",
		"dce850110bd8328cfbd50841d6911d87",
		"67f14984c7da791248e32bb5922583da",
		"1938f2cf72d54ee97e94166fa91d2a36",
		"74481e9646ed49fe0f6224301604698e",
		"57fca5de98a9d6d8006438d0583d8a1d",
		"9fecde1cefdc1cbed4763674d9575359",
		"e3040c00eb28f15366ca73cbd872e740",
		"7697009a6a831dfecca91c5993670f7a",
		"5853542321f567a005d547a4f04759bd",
		"5150d1772f50834a503e069a973fbd7c",
	}
)

// TestVectors compares the hashes of the port with the outputs of
// siphasher, in one call and in writes of each length from 1 to 9 bytes.
func TestVectors(t *testing.T) {
	k0, k1 := uint64(0x0706050403020100), uint64(0x0f0e0d0c0b0a0908)
	cases := []struct {
		name string
		want []string
		one  func([]byte) Hash128
		h    func() hasher128
	}{
		{"13", vectors13, NewSipHasher13WithKeys(k0, k1).Hash, func() hasher128 {
			h := NewSipHasher13WithKeys(k0, k1)
			return &h
		}},
		{"24", vectors24, NewSipHasher24WithKeys(k0, k1).Hash, func() hasher128 {
			h := NewSipHasher24WithKeys(k0, k1)
			return &h
		}},
	}
	var msg []byte
	for i := range 64 {
		for _, c := range cases {
			got := c.one(msg).AsBytes()
			if hex.EncodeToString(got[:]) != c.want[i] {
				t.Errorf("%s, %d bytes: got %x, want %s", c.name, i, got, c.want[i])
			}
			for step := 1; step <= 9; step++ {
				h := c.h()
				for j := 0; j < len(msg); j += step {
					h.Write(msg[j:min(j+step, len(msg))])
				}
				got := h.Finish128().AsBytes()
				if hex.EncodeToString(got[:]) != c.want[i] {
					t.Errorf("%s, %d bytes in writes of %d: got %x, want %s", c.name, i, step, got, c.want[i])
				}
			}
		}
		msg = append(msg, byte(i))
	}
}

// TestIntegers compares the writes of integers with the output of siphasher
// 1.0.2 for the same calls on a 64-bit machine. A Rust program called
// write_u8(1), write_u16(0x0302), write_u32(0x07060504), write(&[8, 9, 10]),
// write_u64(0x1211100f0e0d0c0b), write_usize(0x1a19181716151413) and
// write(&[0x1b; 11]) on SipHasher13::new.
func TestIntegers(t *testing.T) {
	if bits.UintSize != 64 {
		t.Skip("write_usize writes 4 bytes on a 32-bit machine")
	}
	h := NewSipHasher13()
	h.WriteU8(1)
	h.WriteU16(0x0302)
	h.WriteU32(0x07060504)
	h.Write([]byte{8, 9, 10})
	h.WriteU64(0x1211100f0e0d0c0b)
	h.WriteUsize(uint(uint64(0x1a19181716151413) & uint64(^uint(0))))
	h.Write([]byte{0x1b, 0x1b, 0x1b, 0x1b, 0x1b, 0x1b, 0x1b, 0x1b, 0x1b, 0x1b, 0x1b})
	r := h.Finish128()
	b := r.AsBytes()
	if got := hex.EncodeToString(b[:]); got != "c5942807b7df833ecc1350509140104e" {
		t.Errorf("as_bytes gives %s", got)
	}
	if r.H1 != 4504690029492737221 {
		t.Errorf("h1 is %d", r.H1)
	}
	hi, lo := r.AsU128()
	if hi != 0x4e104091505013cc || lo != 0x3e83dfb7072894c5 {
		t.Errorf("as_u128 gives %016x%016x", hi, lo)
	}
	if h.Finish() != 0x4e104091505013cc {
		t.Errorf("finish gives %016x", h.Finish())
	}
	if !Hash128FromU128(hi, lo).Equal(r) {
		t.Error("Hash128FromU128 does not give back the hash")
	}
}
