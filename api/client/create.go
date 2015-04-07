package client

func (cli *DvmClient) DvmCmdCreate(args ...string) error {
	_, _, err := cli.call("POST", "/image/create", nil, nil)
	if err != nil {
		return err
	}
	return nil
}
