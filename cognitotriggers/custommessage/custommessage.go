// package main (custommessage) is a Lambda to handle the Cognito CustomMessage
// trigger: https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-lambda-custom-message.html
// We use this to create our custom email message text for email verification on
// signup, and specifically to handle putting a custom link in that we'll handle
// for this.
// This handler specifies the email subject and body text, and builds the link
// the user will click to do the email verification using the code AWS generates.
// More info can be seen on how all this works in this Stack Overflow:
// https://stackoverflow.com/a/59376006/12876269
package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	stageDev        = "dev"
	stageProduction = "production"

	messageSubject  = "[Echo-Cognito-Auth] Please verify your email"
	messageTemplate = `
<p>Verification code: {####}</p>

<p>Thank you for signing up for Echo-Cognito-Auth.</p>

`
)

var (
	LambdaStage = stageDev // gets set via go build ldflags -X option

	Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

// makeLink is an example of what you might do if you want to add a custom link
// in the verification email, e.g. to a magic link that sends this code into a
// mobile app to automatically do the user verification. Because this sample
// app uses Cognito's managed UI, and doesn't have a mobile app that we could
// use a deep link into, we just return an empty string.
// If you do this, you will need to add a `<A>`/link in your `messageTemplate`
// with the link the user clicks on.
// Note that the {####} will get substitued with the verification code by AWS
// when the email is sent.
func makeLink(codeParam, username string) string {
	// example custom link
	//return fmt.Sprintf("https://mycustomdomain.com/mymobileapp/app/emailverification/?code={####}&user=%s", username)

	return ""
}

// Handler is the main lambda handler. See:
// https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-lambda-custom-message.html
// This uses the details (code, user) in the request portion, and then modifies
// the response struct with our custom email subject and body.
// Also, only handles the signup event, you may want to customize others.
func Handler(event events.CognitoEventUserPoolsCustomMessage) (events.CognitoEventUserPoolsCustomMessage, error) {
	if event.TriggerSource != "CustomMessage_SignUp" {
		return event, nil
	}

	clientID := event.CallerContext.ClientID
	username := event.UserName
	Logger.Info("in CustomMessage handler", "Request", event.Request, "ClientID", clientID, "UserName", username)

	// link := makeLink(code, username)

	event.Response.EmailSubject = messageSubject
	event.Response.EmailMessage = messageTemplate

	return event, nil
}

func main() {
	lambda.Start(Handler)
}
