package entry

import (
	"fmt"
	"log"

	"github.com/anhoder/foxful-cli/util"
	"github.com/go-musicfox/go-musicfox/pkg/commands"
	"github.com/go-musicfox/go-musicfox/pkg/configs"
	"github.com/go-musicfox/go-musicfox/pkg/constants"
	"github.com/go-musicfox/go-musicfox/utils"

	neteaseutil "github.com/go-musicfox/netease-music/util"
	"github.com/gookit/gcli/v2"
)

func runCLI() {
	log.SetOutput(utils.LogWriter())

	var app = gcli.NewApp()
	app.Name = constants.AppName
	app.Version = constants.AppVersion
	app.Description = constants.AppDescription
	app.GOptsBinder = func(gf *gcli.Flags) {
		gf.BoolOpt(&commands.GlobalOptions.PProfMode, "pprof", "p", false, "enable PProf mode")
	}

	// 加载config
	utils.LoadIniConfig()

	util.PrimaryColor = configs.ConfigRegistry.MainPrimaryColor
	var (
		logo         = util.GetAlphaAscii(app.Name)
		randomColor  = util.GetPrimaryColor()
		logoColorful = util.SetFgStyle(logo, randomColor)
	)

	gcli.AppHelpTemplate = fmt.Sprintf(constants.AppHelpTemplate, logoColorful)
	app.Logo.Text = logoColorful

	// 更新netease配置
	neteaseutil.UNMSwitch = configs.ConfigRegistry.UNMSwitch
	neteaseutil.Sources = configs.ConfigRegistry.UNMSources
	neteaseutil.SearchLimit = configs.ConfigRegistry.UNMSearchLimit
	neteaseutil.EnableLocalVip = configs.ConfigRegistry.UNMEnableLocalVip
	neteaseutil.UnlockSoundEffects = configs.ConfigRegistry.UNMUnlockSoundEffects

	var playerCommand = commands.NewPlayerCommand()
	app.Add(playerCommand)
	app.Add(commands.NewConfigCommand())
	app.DefaultCommand(playerCommand.Name)

	app.Run()
}
