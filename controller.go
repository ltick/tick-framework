package ltick

import (
	"github.com/ltick/tick-framework/module"
)

type (
	// 控制器接口
	Controller interface {
		AutoInit(ctx *Context, module *module.Module) Controller
	}
	// 基础控制器
	BaseController struct {
		// 请求上下文
		*Context
		// 所属模块
		Module *module.Module
	}
)

// 自动初始化
func (this *BaseController) AutoInit(ctx *Context, module *module.Module) Controller {
	this.Context = ctx
	this.Module = module
	return this
}
