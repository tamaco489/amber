package dynamodb

import (
	"context"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// fakeAPI は、決めた結果を返すだけの API (呼び出しの中身を残す)
type fakeAPI struct {
	getOut     *ddb.GetItemOutput   // GetItem が返す値
	err        error                // どの操作も、このエラーを返す
	lastPut    *ddb.PutItemInput    // 最後の PutItem の入力
	lastUpdate *ddb.UpdateItemInput // 最後の UpdateItem の入力
}

func (f *fakeAPI) GetItem(context.Context, *ddb.GetItemInput, ...func(*ddb.Options)) (*ddb.GetItemOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.getOut, nil
}

func (f *fakeAPI) PutItem(_ context.Context, in *ddb.PutItemInput, _ ...func(*ddb.Options)) (*ddb.PutItemOutput, error) {
	f.lastPut = in
	if f.err != nil {
		return nil, f.err
	}
	return &ddb.PutItemOutput{}, nil
}

func (f *fakeAPI) UpdateItem(_ context.Context, in *ddb.UpdateItemInput, _ ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error) {
	f.lastUpdate = in
	if f.err != nil {
		return nil, f.err
	}
	return &ddb.UpdateItemOutput{}, nil
}
