package kernel

type OnlyID struct {
	mId int32
}

type WithData struct {
	mId   int32
	mData []byte
}

func (o *OnlyID) OnlyID(mId int32) *OnlyID {
	o.mId = mId
	return o
}

func (o *OnlyID) GetMId() int32 {
	return o.mId
}

func (o *OnlyID) reset() {
	o.mId = 0
}

func (w *WithData) WithData(mId int32, mData []byte) *WithData {
	w.mId = mId
	w.mData = mData
	return w
}

func (w *WithData) GetMId() int32 {
	return w.mId
}

func (w *WithData) GetData() []byte {
	return w.mData
}

func (w *WithData) reset() {
	w.mId = 0
	w.mData = nil
}
