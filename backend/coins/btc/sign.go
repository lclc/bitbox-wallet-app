// Copyright 2018 Shift Devices AG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package btc

import (
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/txsort"
	"github.com/sirupsen/logrus"

	"github.com/digitalbitbox/bitbox-wallet-app/backend/coins/btc/addresses"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/coins/btc/blockchain"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/coins/btc/maketx"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/coins/btc/transactions"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/keystore"
	"github.com/digitalbitbox/bitbox-wallet-app/util/errp"
)

// ProposedTransaction contains all the info needed to sign a btc transaction.
type ProposedTransaction struct {
	TXProposal      *maketx.TxProposal
	PreviousOutputs map[wire.OutPoint]*transactions.SpendableOutput
	GetAddress      func(blockchain.ScriptHashHex) *addresses.AccountAddress
	// Signatures collects the signatures (signatures[transactionInput][cosignerIndex]).
	Signatures [][]*btcec.Signature
	SigHashes  *txscript.TxSigHashes
}

// SignTransaction signs all inputs. It assumes all outputs spent belong to this
// wallet. previousOutputs must contain all outputs which are spent by the transaction.
func SignTransaction(
	keystores keystore.Keystores,
	txProposal *maketx.TxProposal,
	previousOutputs map[wire.OutPoint]*transactions.SpendableOutput,
	getAddress func(blockchain.ScriptHashHex) *addresses.AccountAddress,
	log *logrus.Entry,
) error {
	proposedTransaction := &ProposedTransaction{
		TXProposal:      txProposal,
		PreviousOutputs: previousOutputs,
		GetAddress:      getAddress,
		Signatures:      make([][]*btcec.Signature, len(txProposal.Transaction.TxIn)),
		SigHashes:       txscript.NewTxSigHashes(txProposal.Transaction),
	}

	for i := range proposedTransaction.Signatures {
		proposedTransaction.Signatures[i] = make([]*btcec.Signature, keystores.Count()) // TODO: Replace count with configuration.NumberOfSigners()
	}

	if err := keystores.SignTransaction(proposedTransaction); err != nil {
		return err
	}

	for index, input := range txProposal.Transaction.TxIn {
		spentOutput := previousOutputs[input.PreviousOutPoint]
		address := proposedTransaction.GetAddress(spentOutput.ScriptHashHex())
		input.SignatureScript, input.Witness = address.SignatureScript(
			proposedTransaction.Signatures[index])
	}

	// Sanity check: see if the created transaction is valid.
	if err := txValidityCheck(txProposal.Transaction, previousOutputs,
		proposedTransaction.SigHashes); err != nil {
		log.WithError(err).Panic("Failed to pass transaction validity check.")
	}

	return nil
}

func txValidityCheck(transaction *wire.MsgTx, previousOutputs map[wire.OutPoint]*transactions.SpendableOutput,
	sigHashes *txscript.TxSigHashes) error {
	if !txsort.IsSorted(transaction) {
		return errp.New("tx not bip69 conformant")
	}
	for index, txIn := range transaction.TxIn {
		spentOutput, ok := previousOutputs[txIn.PreviousOutPoint]
		if !ok {
			return errp.New("There needs to be exactly one output being spent per input!")
		}
		engine, err := txscript.NewEngine(spentOutput.PkScript, transaction, index,
			txscript.StandardVerifyFlags, nil, sigHashes, spentOutput.Value)
		if err != nil {
			return errp.WithStack(err)
		}
		if err := engine.Execute(); err != nil {
			return errp.WithStack(err)
		}
	}
	return nil
}
