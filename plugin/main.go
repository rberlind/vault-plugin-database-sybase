package main

import (
	"log"
	"os"

	"github.com/hashicorp/vault/helper/pluginutil"
	plugin "github.com/rberlind/vault-plugin-database-sybase"
)

func main() {
	apiClientMeta := &pluginutil.APIClientMeta{}
	flags := apiClientMeta.FlagSet()
	flags.Parse(os.Args[1:])

	err := plugin.Run(apiClientMeta.GetTLSConfig())
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
