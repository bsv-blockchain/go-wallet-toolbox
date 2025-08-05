package storage

// TODO: test cases:
// 	1. no send change from no send transaction
// 			For storage and wallet:
// 			result11 = CreateAction (noSend)
// 			result12 = ProcessAction(result11)
// 			result21 = CreateAction (noSend, noSendChange = result11.NoSendChange)
// 			result22 = ProcessAction(result21)
// 			assert result11.NoSendChange contains in result21.Inputs
//  2. send with after no send + no send change
//			like 1. + folowing
// 			result31 = CreateAction (sendWith = [result11.TxID, result21.TxID])
// 			result32 = ProcessAction(result31)
// 			assert result31.SendWithResults contains result11.TxID and result21.TxID
// 	3. no send based only on no send changes
// 		don't create action, instead create input and provide it for the result11 = CreateAction (noSend)
//  4. not enough funds with no send changes
//  5. no send change not fully utilized (this could be done in separate task)
//       like 3. but in step result21 = CreateAction (noSend, noSendChange = result11.NoSendChange) no send changes should have more satoshis then would be in output
//       check if after process action not allocated NoSendChange are spendable now
