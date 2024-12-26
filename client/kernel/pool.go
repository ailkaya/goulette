package kernel

import "sync"

var (
	onlyIDPool = sync.Pool{
		New: func() interface{} {
			return &OnlyID{}
		},
	}
	withDataPool = sync.Pool{
		New: func() interface{} {
			return &WithData{}
		},
	}
)

func GetOnlyID(mId int32) *OnlyID {
	// ret := onlyIDPool.Get().(*OnlyID)
	// ret.mId = mId
	// return ret
	return onlyIDPool.Get().(*OnlyID).OnlyID(mId)
}

func PutOnlyID(o *OnlyID) {
	// o.reset()
	onlyIDPool.Put(o)
}

func GetWithData(mId int32, data []byte) *WithData {
	return withDataPool.Get().(*WithData).WithData(mId, data)
}

func PutWithData(w *WithData) {
	// w.reset()
	withDataPool.Put(w)
}
