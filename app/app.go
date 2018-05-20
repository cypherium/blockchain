/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	template "github.com/cypherium_private/mvp"
	"github.com/cypherium_private/mvp/blockchain"
	"github.com/cypherium_private/mvp/blockchain/blkparser"

	"github.com/dedis/onet/app"

	"github.com/dedis/onet/log"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "Template"
	cliApp.Usage = "Used for building other apps."
	cliApp.Version = "0.1"
	groupsDef := "the group-definition-file"
	cliApp.Commands = []cli.Command{
		{
			Name:      "time",
			Usage:     "measure the time to contact all nodes",
			Aliases:   []string{"t"},
			ArgsUsage: groupsDef,
			Action:    cmdTime,
		},
		{
			Name:      "counter",
			Usage:     "return the counter",
			Aliases:   []string{"c"},
			ArgsUsage: groupsDef,
			Action:    cmdCounter,
		},
		{
			Name:      "send",
			Usage:     "send one transaction",
			Aliases:   []string{"s"},
			ArgsUsage: groupsDef,
			Action:    sendTransaction,
		},
		{
			Name:      "check",
			Usage:     "tests if the block is already downloaded, else it will download it.",
			Aliases:   []string{"ck"},
			ArgsUsage: groupsDef,
			Action:    ensureBlockIsAvailable,
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(cliApp.Run(os.Args))
}

func ensureBlockIsAvailable(c *cli.Context) error {
	log.Info("Check command")
	if c.NArg() != 1 {
		log.Fatal("Please give the dir as argument,where to save the blocks")
	}
	dir := c.Args().First()
	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		return errors.New("Couldn't get block: " + err.Error())
	}
	log.Infof("The block is already downloaded.")
	return nil
}

func getTransactions(blocksPath string, nTxs int) ([]blkparser.Tx, error) {
	log.Info("cypherium Client will trigger up to", nTxs, "transactions.", "blocks dir:", blocksPath)
	parser, err := blockchain.NewParser(blocksPath, template.MagicNum)
	if err != nil {
		return nil, errors.New("Couldn't get transactions: " + err.Error())
	}

	transactions, err := parser.Parse(0, template.ReadFirstNBlocks)
	if err != nil {
		return nil, errors.New("Error while parsing transactions " + err.Error())
	}
	if len(transactions) == 0 {
		return nil, fmt.Errorf("Couldn't read any transactions:%v", len(transactions))
	}
	if len(transactions) < nTxs {
		return nil, fmt.Errorf("Read only %v but caller wanted %v", len(transactions), nTxs)
	}
	return transactions, nil
}

//sendTransaction send one transactions,return the result
func sendTransaction(c *cli.Context) error {
	log.Info("Send command")
	if c.NArg() != 2 {
		log.Fatal("Please give the public.toml file and give the transactions number as second argument")
	}
	nTxs, err := strconv.Atoi(c.Args().Get(1))
	group := readGroup(c)
	client := template.NewClient()
	//transactions, err := getTransactions(dir, nTxs)
	transactions := GetStxs(nTxs)
	if err != nil {
		return err
	}
	resp, err := client.Send(group.Roster, transactions[:nTxs])
	if err != nil {
		return errors.New("When asking the transaction: " + err.Error())
	}
	log.Infof("Children: %d - Time spent: %f.send result:%v", resp.Children, resp.Time, resp.Status)
	return nil
}

// Returns the time needed to contact all nodes.
func cmdTime(c *cli.Context) error {
	log.Info("Time command")
	group := readGroup(c)
	client := template.NewClient()
	resp, err := client.Clock(group.Roster)
	if err != nil {
		return errors.New("When asking the time: " + err.Error())
	}
	log.Infof("Children: %d - Time spent: %f", resp.Children, resp.Time)
	return nil
}

// Returns the number of calls.
func cmdCounter(c *cli.Context) error {
	log.Info("Counter command")
	group := readGroup(c)
	client := template.NewClient()
	counter, err := client.Count(group.Roster.RandomServerIdentity())
	if err != nil {
		return errors.New("When asking for counter: " + err.Error())
	}
	log.Info("Number of requests:", counter)
	return nil
}

func readGroup(c *cli.Context) *app.Group {
	name := c.Args().First()
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}
