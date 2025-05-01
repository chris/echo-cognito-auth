// package main (postconfirmation) - is a Lambda to handle creating a user in
// in our application from a Cognito post-confirmation event (i.e. user fully
// signed up).
package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	stageDev        = "dev"
	stageProduction = "production"
)

var (
	LambdaStage = stageDev // gets set via go build ldflags -X option

	Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

// This would be defined in your application models
type User struct {
	UserID string
}

func createUser(userID string) (User, error) {
	// TODO: do whatever you need to create the user in your system
	return User{UserID: userID}, nil
}

// updateCognitoUser updates the Cognito user record with custom attributes.
// Custom attribute names are of the format "custom:attrName".
func updateCognitoUser(cognitoUserPoolId string, user User) error {
	// Use this if applicable

	// cfg := cognito.NewCognitoUserConfig(cognitoUserPoolId, user.UserID)
	// return cfg.UpdateAttributes(map[string]string{
	// 	customAttrName1: user.SomeValue1,
	// 	customAttrName2: user.SomeValue2,
	// })

	return nil
}

// Handler is the lambda entry point that handles the Cognito Post Confirmation
// event. For Cognito post confirm events, we need to respond with the existing
// event (no modifications needed). See:
// https://github.com/aws/aws-lambda-go/blob/master/events/README_Cognito_UserPools_PostConfirmation.md
// Also, this method *could* change aspects of the signup if it needed to, but
// that isn't our need. AWS docs:
// https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-lambda-post-confirmation.html#aws-lambda-triggers-post-confirmation-example
//
// !!! Note that this is called after the user has confirmed, so they have a valid
// account in Cognito. Returning an error from this method does not fail their
// signup, but should return errors anytime there are ones we want to know about,
// as logging and/or monitoring will get those.
// The trigger should get retried if there is an error, so beware of that,
// which means this method should be idempotent.
// Note 2: this lambda is triggered for PostConfirmation events, of which there
// are two: signup and forgot password. We only respond to signup. See:
// https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-user-identity-pools-working-with-aws-lambda-triggers.html#cognito-user-identity-pools-working-with-aws-lambda-trigger-sources
// fmt.Printf("Full Cognito event: %+v\n", event)
func Handler(event events.CognitoEventUserPoolsPostConfirmation) (events.CognitoEventUserPoolsPostConfirmation, error) {
	// The Cognito username is the same as the "sub" attribute
	if event.TriggerSource != "PostConfirmation_ConfirmSignUp" {
		return event, nil
	}

	userName := event.UserName
	userAttribs := event.Request.UserAttributes
	userEmail := userAttribs["email"]
	Logger.Info("Cognito data",
		"userPoolID", event.UserPoolID,
		"username", userName,
		"email", userEmail,
		"sub", userAttribs["sub"],
		"attributes", userAttribs)

	user, err := createUser(userName)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate user") {
			// This is a duplicate user, so we presume this got double called,
			// and just ignore it and return
			return event, nil
		}
		Logger.Error("Failed to create user", "error", err)
		return event, err
	}
	Logger.Info("Created new user from Cognito", "userID", userName)

	err = updateCognitoUser(event.UserPoolID, user)
	if err != nil {

		Logger.Error("Failed to update Cognito user access token", "error", err)
	}
	Logger.Info("Updated Cognito user.")

	if LambdaStage == stageProduction {
		// You could do more here, only for a production deployment, such as adding
		// a user to a mailing list or other setup aspects for your app. Just ensure
		// these don't create an error if they aren't required to succeed.
	}

	return event, err
}

func main() {
	lambda.Start(Handler)
}
