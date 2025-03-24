package proto

import (
	"fmt"

	utils "github.com/jeetdotso/tweet-indexer-service/persistence/util"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// AnyMapToProtoMap convert map of interface{} to map of proto any
// Casts everything to string except bool type
func AnyMapToProtoMap(anyMap map[string]interface{}) map[string]*anypb.Any {
	protoAnyMap := make(map[string]*anypb.Any)

	for k, v := range anyMap {
		switch v.(type) {
		case *utils.MBigInt:
			v = v.(*utils.MBigInt).BigInt.String()
			msg := wrapperspb.String(v.(string))
			newAny, err := anypb.New(msg)
			if err != nil {
				fmt.Printf("Error on creating new proto any: %s\n", err)
			}
			protoAnyMap[k] = newAny
		case bool:
			msg := wrapperspb.Bool(v.(bool))
			newAny, err := anypb.New(msg)
			if err != nil {
				fmt.Printf("Error on creating new proto any: %s\n", err)
			}
			protoAnyMap[k] = newAny
		default:
			v = fmt.Sprint(v)
			msg := wrapperspb.String(v.(string))
			newAny, err := anypb.New(msg)
			if err != nil {
				fmt.Printf("Error on creating new proto any: %s\n", err)
			}
			protoAnyMap[k] = newAny
		}
	}

	return protoAnyMap
}
