package boltz

// Normal (submarine) swap finite state machine

func FsmInitialNormal(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

func FsmOnChainFundsSent(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

func FsmRedeemLockedFunds(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

func FsmRedeemingLockedFunds(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

func FsmVerifyFundsReceived(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

/*
func (b *Plugin) InitNormalSwap(sd types.SwapData, msgCallback entities.MessageCallback) {
	const BlockEps = 10

	keys, err := b.GetKeys(id)
	if err != nil {
		b.Fail(sd, err, msgCallback)
		return
	}

	lnAPI, err := b.LnAPI()
	if err != nil {
		b.Fail(sd, err, msgCallback)
		return
	}
	if lnAPI == nil {
		b.Fail(sd, fmt.Errorf("error checking lightning"), msgCallback)
		return
	}
	defer lnAPI.Cleanup()

	ctx := context.Background()
	info, err := lnAPI.GetInfo(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Doing swap from %s\n", info.Alias)

	invoice, err := lnAPI.CreateInvoice(ctx, 40000, hex.EncodeToString(keys.Preimage.Hash), "", 24*time.Hour)
	if err != nil {
		fmt.Printf("Invoice error: %v", err)
		return err
	}

	fmt.Printf("Invoice %s %s\n", invoice.Hash, invoice.PaymentRequest)

	response, err := b.BoltzAPI.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          "BTC/BTC",
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(keys.Preimage.Hash),
		RefundPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
		Invoice:         invoice.PaymentRequest,
	})

	if err != nil {
		fmt.Printf("Error creating swap: %v", err)
		return err
	}

	fmt.Printf("Response: %+v\n", response)

	redeemScript, err := hex.DecodeString(response.RedeemScript)
	if err != nil {
		return err
	}

	fmt.Printf("Timeout %v\n", response.TimeoutBlockHeight)

	err = boltz.CheckSwapScript(redeemScript, keys.Preimage.Hash, keys.Keys.PrivateKey, response.TimeoutBlockHeight)
	if err != nil {
		return err
	}

	err = boltz.CheckSwapAddress(b.ChainParams, response.Address, redeemScript, true)
	if err != nil {
		return err
	}

	if info.BlockHeight+BlockEps < int(response.TimeoutBlockHeight) {
		return fmt.Errorf("error checking blockheight")
	}

}
*/
