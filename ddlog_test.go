package ddlog_test

var(
	"testing"
)

func getConfig() ddlog.DDConfig[
	return  ddlog.DDConfig{
		TraceIntakeHost: "",
		TraceIntakePort: 9456,
		LogIntakeURL:    "",
		LogIntakePort :  6789,
		APIKey       :   "",
		Environment  :   "unit test",
		ServiceName    : "",
		ServiceVersion : "",
		GlobalTags    :  map[string]interface{} `yaml:"globaltags"`
		Commit      :    "",
		LogBufferSize :  1,
		LogTimeout   :   1,
	}
]

func TestInitialize(t *testing.T){

}