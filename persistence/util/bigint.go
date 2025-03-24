package util

import (
	"errors"
	"fmt"
	"github.com/jeetdotso/tweet-indexer-service/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"math/big"
)

type MBigInt struct {
	BigInt *big.Int
}

func NewMBigInt(bint *big.Int) *MBigInt {
	return &MBigInt{BigInt: bint}
}

func ParseBigInt(s string) (*big.Int, error) {
	bigint := new(big.Int)
	bigint, ok := bigint.SetString(s, 10)
	if !ok {
		fmt.Println("SetString: error")
		return nil, errors.New("error on converting string in big int")
	}
	return bigint, nil
}

func ParseBigIntOrDefault(s string) *big.Int {
	b, err := ParseBigInt(s)
	if err != nil {
		fmt.Println(err.Error())
		return big.NewInt(0)
	}
	return b
}

// ParseArrBigInt Parse big int array into m big int arr
func ParseArrBigInt(from []*big.Int) []*MBigInt {
	var to []*MBigInt
	for _, b := range from {
		to = append(to, NewMBigInt(b))
	}
	return to
}

func ParseMBigIntOrDefault(s string) *MBigInt {
	return NewMBigInt(ParseBigIntOrDefault(s))
}

func (i *MBigInt) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(i.BigInt.String())
}

func (i *MBigInt) UnmarshalBSONValue(t bsontype.Type, value []byte) error {
	if t != bsontype.String {
		return fmt.Errorf("invalid bson value type '%s'", t.String())
	}
	str, _, ok := bsoncore.ReadString(value)
	if !ok {
		return fmt.Errorf("invalid bson string value")
	}
	bigint, err := util.StringToBigInt(str)
	if err != nil {
		return fmt.Errorf("error converting string to bigint ('%s')", err)
	}
	i.BigInt = bigint
	return nil
}
