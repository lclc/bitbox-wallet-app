package maketx

import (
	"sort"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/txsort"
	"github.com/shiftdevices/godbb/coins/btc/addresses"
	"github.com/shiftdevices/godbb/util/errp"
	"github.com/sirupsen/logrus"
)

// TxProposal is the data needed for a new transaction to be able to display it and sign it.
type TxProposal struct {
	// Amount is the amount that is sent out. The fee is not included and is deducted on top.
	Amount btcutil.Amount
	// Fee is the mining fee used.
	Fee         btcutil.Amount
	Transaction *wire.MsgTx
	// ChangeAddress is the address of the wallet to which the change of the transaction is sent.
	ChangeAddress *addresses.Address
}

type byValue struct {
	outPoints []wire.OutPoint
	outputs   map[wire.OutPoint]*wire.TxOut
}

func (p *byValue) Len() int { return len(p.outPoints) }
func (p *byValue) Less(i, j int) bool {
	return p.outputs[p.outPoints[i]].Value < p.outputs[p.outPoints[j]].Value
}
func (p *byValue) Swap(i, j int) { p.outPoints[i], p.outPoints[j] = p.outPoints[j], p.outPoints[i] }

func coinSelection(
	minAmount btcutil.Amount,
	outputs map[wire.OutPoint]*wire.TxOut,
	log *logrus.Entry,
) (btcutil.Amount, []wire.OutPoint, error) {
	outPoints := []wire.OutPoint{}
	for outPoint := range outputs {
		outPoints = append(outPoints, outPoint)
	}
	sort.Sort(sort.Reverse(&byValue{outPoints, outputs}))
	selectedOutPoints := []wire.OutPoint{}
	outputsSum := btcutil.Amount(0)

	for _, outPoint := range outPoints {
		if outputsSum >= minAmount {
			break
		}
		selectedOutPoints = append(selectedOutPoints, outPoint)
		outputsSum += btcutil.Amount(outputs[outPoint].Value)
	}
	if outputsSum < minAmount {
		return 0, nil, errp.WithContext(errp.New("Insufficient funds"),
			errp.Context{"min-amount": minAmount})
	}
	return outputsSum, selectedOutPoints, nil
}

// NewTxSpendAll creates a transaction which spends all available unspent outputs.
func NewTxSpendAll(
	spendableOutputs map[wire.OutPoint]*wire.TxOut,
	outputPkScript []byte,
	feePerKb btcutil.Amount,
	log *logrus.Entry,
) (*TxProposal, error) {

	selectedOutPoints := []wire.OutPoint{}
	inputs := []*wire.TxIn{}
	outputsSum := btcutil.Amount(0)
	for outPoint, output := range spendableOutputs {
		outPoint := outPoint // avoid reference reuse due to range loop
		selectedOutPoints = append(selectedOutPoints, outPoint)
		outputsSum += btcutil.Amount(output.Value)
		inputs = append(inputs, wire.NewTxIn(&outPoint, nil, nil))
	}
	output := wire.NewTxOut(0, outputPkScript)
	txSize := EstimateSerializeSize(len(selectedOutPoints), []*wire.TxOut{output}, false)
	maxRequiredFee := FeeForSerializeSize(feePerKb, txSize, log)
	if outputsSum < maxRequiredFee {
		return nil, errp.New("Insufficient funds for fee")
	}
	output = wire.NewTxOut(int64(outputsSum-maxRequiredFee), outputPkScript)
	unsignedTransaction := &wire.MsgTx{
		Version:  wire.TxVersion,
		TxIn:     inputs,
		TxOut:    []*wire.TxOut{output},
		LockTime: 0,
	}
	txsort.InPlaceSort(unsignedTransaction)
	log.WithField("fee", maxRequiredFee).Debug("Preparing transaction to spend all outputs")
	return &TxProposal{
		Amount:      btcutil.Amount(output.Value),
		Fee:         maxRequiredFee,
		Transaction: unsignedTransaction,
	}, nil
}

// NewTx creates a transaction from a set of unspent outputs, targeting an output value. A subset of
// the unspent outputs is selected to cover the needed amount. A change output is added if needed.
func NewTx(
	spendableOutputs map[wire.OutPoint]*wire.TxOut,
	output *wire.TxOut,
	feePerKb btcutil.Amount,
	getChangeAddress func() *addresses.Address,
	log *logrus.Entry,
) (*TxProposal, error) {
	targetAmount := btcutil.Amount(output.Value)
	outputs := []*wire.TxOut{output}
	estimatedSize := EstimateSerializeSize(1, outputs, true)
	targetFee := FeeForSerializeSize(feePerKb, estimatedSize, log)

	for {
		selectedOutputsSum, selectedOutPoints, err := coinSelection(
			targetAmount+targetFee,
			spendableOutputs,
			log,
		)
		if err != nil {
			return nil, err
		}

		txSize := EstimateSerializeSize(len(selectedOutPoints), outputs, true)
		maxRequiredFee := FeeForSerializeSize(feePerKb, txSize, log)
		if selectedOutputsSum-targetAmount < maxRequiredFee {
			targetFee = maxRequiredFee
			continue
		}

		inputs := make([]*wire.TxIn, len(selectedOutPoints))
		for i, outPoint := range selectedOutPoints {
			inputs[i] = wire.NewTxIn(&outPoint, nil, nil)
		}
		unsignedTransaction := &wire.MsgTx{
			Version:  wire.TxVersion,
			TxIn:     inputs,
			TxOut:    outputs,
			LockTime: 0,
		}
		changeAmount := selectedOutputsSum - targetAmount - maxRequiredFee
		changeIsDust := IsDustAmount(changeAmount, P2PKHPkScriptSize, feePerKb)
		finalFee := maxRequiredFee
		if changeIsDust {
			finalFee = selectedOutputsSum - targetAmount
		}
		var changeAddress *addresses.Address
		if changeAmount != 0 && !changeIsDust {
			changeAddress = getChangeAddress()
			changePKScript := changeAddress.PkScript()
			if len(changePKScript) > P2PKHPkScriptSize {
				return nil, errp.WithContext(errp.New("fee estimation requires change scripts no "+
					"larger than P2PKH output scripts"),
					errp.Context{"change-script-size": len(changePKScript)})
			}
			changeOutput := wire.NewTxOut(int64(changeAmount), changePKScript)
			unsignedTransaction.TxOut = append(unsignedTransaction.TxOut, changeOutput)
		}
		txsort.InPlaceSort(unsignedTransaction)
		log.WithField("fee", finalFee).Debug("Preparing transaction")
		return &TxProposal{
			Amount:        targetAmount,
			Fee:           finalFee,
			Transaction:   unsignedTransaction,
			ChangeAddress: changeAddress,
		}, nil
	}
}
