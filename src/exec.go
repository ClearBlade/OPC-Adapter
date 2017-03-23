package dukdukgo

//

import (
	"clearblade/go-duktape"
	"clearblade/uuid"
	"encoding/json"
	"fmt"
	"strings"
	//	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type ExecutionEngine struct {
	systemkey, systemsecret, userToken string
	httpclient                         *http.Client
	currentRequest                     *http.Request
	ctx                                *duktape.Context
	httpMut                            *sync.Mutex
}

func NewExecutionEngine(systemkey, systemsecret, usertoken string) *ExecutionEngine {
	return &ExecutionEngine{
		systemkey:    systemkey,
		systemsecret: systemsecret,
		userToken:    usertoken,
		httpMut:      new(sync.Mutex),
		//ctx: duktape.NewContext(),
		ctx:        duktape.GetSafeContextWithDestructionHandler(),
		httpclient: &http.Client{},
	}
}
func (ee *ExecutionEngine) SupplyRequestAndResponseObjects() {
	str := fmt.Sprintf(`var _request = {};_request.params = {};_request.systemKey = "%s";_request.systemSecret = "%s";_request.userToken="%s"`, ee.systemkey, ee.systemsecret, ee.userToken)
	ee.ctx.PevalString(str)
	str = `var _response = {};_response.success = _success;_response.error = _success`
	ee.ctx.PevalString(str)
}

func genParam(ctx *duktape.Context, m map[string]interface{}) error {
	params := ""
	for k, v := range m {
		switch v.(type) {
		case bool:
			if v.(bool) {
				params += fmt.Sprintf("_request.params.%s = true;", k)
			} else {
				params += fmt.Sprintf("_request.params.%s = false;", k)
			}
		case float64:
			f := v.(float64)
			params += fmt.Sprintf("_request.params.%s = %f;", k, f)
		case string:
			params += fmt.Sprintf(`_request.params.%s = "%s";`, k, v.(string))
		case map[string]interface{}:
			byts, err := json.Marshal(v.(map[string]interface{}))
			if err != nil {
				return err
			}
			params += fmt.Sprintf(`_request.params.%s = %s;`, k, byts)
		case []interface{}, []float64, []string, []map[string]interface{}:
			byts, err := json.Marshal(v)
			if err != nil {
				return err
			}
			params += fmt.Sprintf(`_request.params.%s = %s;`, k, byts)
		default:
			log.Printf("got type %T  %+v \n", v, v)
		}
	}

	i := ctx.PevalString(params)
	if i != 0 {
		return fmt.Errorf(ctx.SafeToString(-1))
	} else {
		return nil
	}
}

func (ee *ExecutionEngine) Kill() {
	ee.httpMut.Lock()
	if ee.currentRequest != nil {
		ee.httpclient.Transport.(*http.Transport).CancelRequest(ee.currentRequest)
	}
	ee.ctx.RemoveStoredFuncs()
	ee.ctx.DestroyHeap()
	ee.httpMut.Unlock()
	runtime.Goexit()
}

func (ee *ExecutionEngine) supplyNativeFuncs(libNames []string) {
	//the first funcs are supplied to every call
	ee.ctx.PushGoFunc("log", func(ctx *duktape.Context) int {
		arg := ctx.GetString(-1)
		log.Println(arg)
		ctx.Pop()
		return 0
	})

	for _, name := range libNames {
		switch strings.ToLower(name) {
		case "clearblade":
			ee.ctx.PushGoFunc("getMessageHistory", getMessageHistory)
			ee.ctx.PushGoFunc("loginAnon", loginAnonFromJS)
			ee.ctx.PushGoFunc("regUser", registerUserFromJS)
			ee.ctx.PushGoFunc("isCurrentUserAuthed", isCurrentUserAuthedFromJS)
			ee.ctx.PushGoFunc("logUser", loginUserFromJS)
			ee.ctx.PushGoFunc("logoutUser", logoutUserFromJS)
			ee.ctx.PushGoFunc("resolveQueryName", resolveQueryName)
			ee.ctx.PushGoFunc("performSingleUsersQuery", performSingleUsersQuery)
			ee.ctx.PushGoFunc("performDeleteQuery", performDeleteQuery)
			ee.ctx.PushGoFunc("performSelectQuery", performSelectQuery)
			ee.ctx.PushGoFunc("performInsertQuery", performInsertQuery)
			ee.ctx.PushGoFunc("publishMessage", publishMessage)
			ee.ctx.PushGoFunc("performUpdateQuery", performUpdateQuery)
			ee.ctx.PushGoFunc("setUserInfo", setUserInfo)
			ee.ctx.PushGoFunc("getAllUsers", getAllUsers)
			ee.ctx.PushGoFunc("saveItem", saveItem)
		case "http":
			//we're dropping this into a heap allocated function so
			//that it can capture the pointer to the ee object
			//this is important so that we can keep track of
			//whether or not the http resquestor is closed or open
			//because it' it's open and we kill the goroutine
			//it'll cause a panic and crash the runtime
			thunk := func(d *duktape.Context) int {
				res := ee.httpcall(d)
				return res
			}
			ee.ctx.PushGoFunc("httpcall", thunk)
		case "mailer":
			ee.ctx.PushGoFunc("sendEmailCall", sendEmailCall)
		}
	}

}

func (ee *ExecutionEngine) DoExecution(reqid uuid.Uuid, myCode, myCodeName string, libNames []string, args map[string]interface{}) map[string]interface{} {
	resp := make(chan interface{})
	go func() {
		defer func() {
			if recover() != nil {
				resp <- "error: unknown error"
			}
		}()

		ee.ctx.PushGoFunc("_trap", func(ctx *duktape.Context) int {
			defer func() {
				if r := recover(); r != nil { /*noop*/
				}
			}()
			ee.Kill()
			return 0
		})
		ee.ctx.PushGoFunc("_success", func(ctx *duktape.Context) int {
			defer func() {
				if r := recover(); r != nil { /*noop*/
				}
			}()
			if ctx.IsObject(-1) {
				ctx.JsonEncode(-1)
				o := ctx.GetString(-1)
				m := make(map[string]interface{})
				//what can I do about an error at this point?
				err := json.Unmarshal([]byte(o), &m)
				if err != nil {
					//maybe it's an array, but js calls it an object because who the hell knows
					var ma []interface{}
					err = json.Unmarshal([]byte(o), &ma)
					if err != nil {
						resp <- fmt.Sprintf("error formatting return value %s\n", err.Error())
					}
					resp <- ma
				}

				resp <- m
			} else if ctx.IsArray(-1) {
				ctx.JsonEncode(-1)
				o := ctx.GetString(-1)
				ma := make([]interface{}, 0)
				_ = json.Unmarshal([]byte(o), &ma)
				//can't do anything for error at this point, may as well send empty
				resp <- ma
			} else if ctx.IsString(-1) {
				out := ctx.GetString(-1)
				resp <- out
			} else if ctx.IsNumber(-1) {
				out := ctx.GetNumber(-1)
				resp <- out
			} else if ctx.IsBoolean(-1) {
				out := ctx.GetBoolean(-1)
				resp <- out
			} else {
				out := "invalid response type"
				resp <- out
			}
			ee.Kill()
			return 0
		})

		ee.SupplyRequestAndResponseObjects()
		//		str := `var _response = {};_response.success = _success;_response.error = _success;`
		err := genParam(ee.ctx, args)
		if err != nil {
			resp <- err.Error()
			return
		}
		ee.supplyNativeFuncs(libNames)
		ee.ctx.PevalString(fmt.Sprintf("var REQUESTID = \"%s\"", reqid.String()))
		j := ee.ctx.PevalString(clearbladejs2)
		if j != 0 {
			resp <- "cbjs " + ee.ctx.SafeToString(-1)
		}
		j = ee.ctx.PevalString(clearbladejsRequest)
		if j != 0 {
			resp <- "req " + ee.ctx.SafeToString(-1)
		}

		j = ee.ctx.PevalString(mailerjs)
		if j != 0 {
			resp <- "mailer " + ee.ctx.SafeToString(-1)
		}
		ee.ctx.PevalString(`var ClearBlade = new cbLib()`)
		ee.ctx.PevalString(requestHandler)
		go func(c *duktape.Context, s chan interface{}) {
			defer func() {
				if r := recover(); r != nil {
					//todo: get an error message
					buf := make([]byte, 2048)
					runtime.Stack(buf, false)
					log.Println(string(buf))
					select {
					case s <- "failure to execute":
					default:
					}
				}
			}()

			//TODO: IS THIS NESSESSARY??!?!?!? -- swm: NOOOOOO
			//myCode = strings.Replace(myCode, "\\n", "\n", -1)
			i := c.PevalString(myCode)
			if i != 0 {
				s <- c.SafeToString(-1)
			}

			i = c.PevalString("try{ " + myCodeName + "(_request,_response)" + `}catch(e){_success(e+"")}`)
			if i != 0 {
				s <- c.SafeToString(-1)
			}
		}(ee.ctx, resp)

	}()
	out := make(map[string]interface{})
	select {
	case msg := <-resp:
		out["results"] = msg
		return out
	case <-time.After(30 * time.Second):
		log.Println("timing out")
		//there has to be a better way of doing this.
		//but this basically just pulls the carpet out
		//from underneath the executing function
		//so that it crashes inside whenever it tries to adjust a heap object or whatever
		//but a `while(true){}` call will probably block forever.
		//however, it seems impossible to reach into
		//the code and stop it
		ee.Kill()
		return map[string]interface{}{"results": "time out"}
	}
}
