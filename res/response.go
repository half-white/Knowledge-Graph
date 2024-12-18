package res

import (
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type Response struct {
	Code int    `json:"code"`
	Data any    `json:"data"`
	Msg  string `json:"msg"`
}

type ListResponse[T any] struct {
	Count int64 `json:"count"`
	List  T     `json:"list"`
}

const (
	Success = 0
	Error   = 7
)

// 调用响应，实例化
func Result(code int, data any, msg string, c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Data: data,
		Msg:  msg,
	})
}

// 响应成功
func Ok(data any, msg string, c *gin.Context) {
	Result(Success, data, msg, c)
}

func OkWithData(data any, c *gin.Context) {
	Result(Success, data, "成功", c)
}

func OkWithList(list any, count int64, c *gin.Context) {
	OkWithData(ListResponse[any]{
		List:  list,
		Count: count,
	}, c)
}

func OkWithMessage(msg string, c *gin.Context) {
	Result(Success, map[string]any{}, msg, c)
}

func OkWith(c *gin.Context) {
	Result(Success, map[string]any{}, "成功", c)
}

// 响应失败
func Fail(data any, msg string, c *gin.Context) {
	Result(Error, data, msg, c)
}

func FailWithMessage(msg string, c *gin.Context) {
	Result(Error, map[string]any{}, msg, c)
}

func FailWithError(err error, obj any, c *gin.Context) {
	msg := GetVaildMsg(err,obj)
	FailWithMessage(msg,c)
}

func FailWithCode(code ErrorCode,c *gin.Context) {
	msg,ok := ErrorMap[code]
	if ok {
		Result(int(code),map[string]any{},msg,c)
		return
	}
	Result(Error,map[string]any{},msg,c)
}

func GetVaildMsg(err error, obj any) string {
	//使用的时候，需要传递obj的指针
	getobj := reflect.TypeOf(obj)

	//将err接口断言为具体类型
	if errs, ok := err.(validator.ValidationErrors); ok {
		//断言成功
		for _, e := range errs {
			//循环每一个错误信息
			//根据报错字段名，获取结构体具体字段
			if f, exits := getobj.Elem().FieldByName(e.Field()); exits {
				msg := f.Tag.Get("msg")
				return msg
			}
		}
	}
	return err.Error()
}
