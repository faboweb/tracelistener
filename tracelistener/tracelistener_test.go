//go:build !race
// +build !race

package tracelistener_test

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	models "github.com/allinbits/demeris-backend-models/tracelistener"
	"github.com/allinbits/tracelistener/tracelistener"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOperation_String(t *testing.T) {
	require.Equal(t, "write", tracelistener.WriteOp.String())
}

type testDatabaseEntrier struct {
	cn string
}

func (t testDatabaseEntrier) WithChainName(cn string) models.DatabaseEntrier {
	t.cn = cn
	return t
}

func TestWritebackOp_InterfaceSlice(t *testing.T) {
	tests := []struct {
		name   string
		fields []models.DatabaseEntrier
		want   []interface{}
	}{
		{
			"slice with single objects are equal",
			[]models.DatabaseEntrier{
				testDatabaseEntrier{cn: "cn"},
			},
			[]interface{}{
				testDatabaseEntrier{cn: "cn"},
			},
		},
		{
			"slice with multiple objects are equal",
			[]models.DatabaseEntrier{
				testDatabaseEntrier{cn: "cn"},
				testDatabaseEntrier{cn: "cn2"},
			},
			[]interface{}{
				testDatabaseEntrier{cn: "cn"},
				testDatabaseEntrier{cn: "cn2"},
			},
		},
		{
			"empty slice yields an empty one",
			[]models.DatabaseEntrier{},
			[]interface{}{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wb := tracelistener.WritebackOp{
				Data: tt.fields,
			}

			require.Equal(t, tt.want, wb.InterfaceSlice())
		})
	}
}

func TestTraceWatcher_Watch(t *testing.T) {
	op := `{"operation":"write","key":"aWJjL2Z3ZC8weGMwMDA0ZThkMzg=","value":"cG9ydHMvdHJhbnNmZXI=","metadata":null}`
	tests := []struct {
		name        string
		ops         []tracelistener.Operation
		data        string
		wantErr     bool
		differentOp bool
		shouldPanic bool
		errSent     error
	}{
		{
			"write operation is configured and read accordingly",
			[]tracelistener.Operation{
				tracelistener.WriteOp,
			},
			op,
			false,
			false,
			false,
			nil,
		},
		{
			"write operation is not configured and not read",
			[]tracelistener.Operation{
				tracelistener.ReadOp,
			},
			op,
			false,
			true,
			false,
			nil,
		},
		{
			"any operation is configured and read accordingly",
			[]tracelistener.Operation{},
			op,
			false,
			false,
			false,
			nil,
		},
		{
			"an EOF doesn't impact anything",
			[]tracelistener.Operation{},
			op,
			false,
			false,
			false,
			io.EOF,
		},
		{
			"a random error panics",
			[]tracelistener.Operation{},
			op,
			true,
			false,
			true,
			fmt.Errorf("error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "test_data")
			require.NoError(t, err)

			defer os.Remove(f.Name())

			dataChan := make(chan tracelistener.TraceOperation)
			errChan := make(chan error)
			l, _ := zap.NewDevelopment()
			tw := tracelistener.TraceWatcher{
				DataSourcePath: f.Name(),
				WatchedOps:     tt.ops,
				DataChan:       dataChan,
				ErrorChan:      errChan,
				Logger:         l.Sugar(),
			}

			go func() {
				if tt.shouldPanic {
					require.Panics(t, func() {
						tw.Watch()
					})
				} else {
					tw.Watch()
				}
			}()

			if tt.errSent != nil && !tt.shouldPanic {

				tw.DataSourcePath = f.Name()
			}

			n, err := f.Write([]byte(tt.data))
			require.NoError(t, err)

			if !tt.shouldPanic {
				require.NoError(t, err)
				require.Equal(t, len(tt.data), n)

				if tt.wantErr {
					require.Error(t, <-errChan)
					return
				}

				if !tt.differentOp {
					require.Eventually(t, func() bool {
						d := <-dataChan
						return d.Key != nil
					}, time.Second, 10*time.Millisecond)
					return
				}

				require.Never(t, func() bool {
					d := <-dataChan
					return d.Key != nil
				}, time.Second, 10*time.Millisecond)
			}
		})
	}
}

func TestWritebackOp_SplitStatements(t *testing.T) {
	tests := []struct {
		name           string
		needle         tracelistener.WritebackOp
		limit          int
		expectedAmount int64
		mustPanic      bool
	}{
		{
			"limit equal to (fieldsAmount*4 - 1), returns 4 elements",
			tracelistener.WritebackOp{
				DatabaseExec: "",
				Data: []models.DatabaseEntrier{
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
				},
			},
			15,
			4,
			false,
		},
		{
			"limit of fieldsAmount returns exactly len(needle.Data)",
			tracelistener.WritebackOp{
				DatabaseExec: "statement",
				Data: []models.DatabaseEntrier{
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
				},
			},
			4,
			4,
			false,
		},
		{
			"limit greater than fieldsAmount*4 returns exactly 1 element",
			tracelistener.WritebackOp{
				DatabaseExec: "statement",
				Data: []models.DatabaseEntrier{
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
					models.AuthRow{
						TracelistenerDatabaseRow: models.TracelistenerDatabaseRow{
							ChainName: "chain",
						},
						Address:        "address",
						SequenceNumber: 1,
						AccountNumber:  1,
					},
				},
			},
			40,
			1,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panicf := require.Panics
			if !tt.mustPanic {
				panicf = require.NotPanics
			}

			val := []tracelistener.WritebackOp{}
			panicf(t, func() {
				val = tt.needle.SplitStatements(tt.limit)
			})

			require.Len(t, val, int(tt.expectedAmount))
		})
	}
}
