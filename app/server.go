// package main - the main file for Ride All the Trails web app.
package main

import (
	"embed"
	"encoding/gob"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	slogecho "github.com/samber/slog-echo"

	"echo-cognito-auth/models"
	"echo-cognito-auth/views"
)

const (
	sessionName    = "session"
	sessionUserKey = "user"
	contextUserKey = "user"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

//go:embed assets
var staticAssets embed.FS

type CustomContext struct {
	echo.Context
}

func (c *CustomContext) User() *models.User {
	user := c.Get(contextUserKey)
	if user != nil {
		return user.(*models.User)
	}
	return nil
}

func main() {
	app := echo.New()

	setupMiddleware(app)
	setupRoutes(app)

	// Use port 8080 as this is the default port for AWS Lambda Web Adapter
	app.Logger.Fatal(app.Start(":8080"))
}

func setupMiddleware(e *echo.Echo) {
	e.Use(slogecho.New(logger))
	e.Use(middleware.Recover())

	// session middleware & register custom types stored in session
	store := sessions.NewCookieStore([]byte(os.Getenv("ECHO_COGNITO_AUTH_SESSION_SECRET")))
	e.Use(session.Middleware(store))

	gob.Register(models.User{})

	// This needs the session, so needs to be after session middleware
	e.Use(AddUserToContext)
}

func setupRoutes(e *echo.Echo) {
	useOS := len(os.Args) > 1 && os.Args[1] == "live"
	assetHandler := http.FileServer(getFileSystem(useOS))
	e.GET("/assets/*", echo.WrapHandler(http.StripPrefix("/assets/", assetHandler)))

	e.GET("/", HomeHandler)
	e.GET("/login", LoginHandler)
	e.GET("/auth/cognito/callback", CognitoCallbackHandler)
	e.GET("/logout", LogoutHandler)

	// Protected routes
	adminGroup := e.Group("/admin", RequireAuth)
	adminGroup.GET("", AdminHandler)

	userGroup := e.Group("/user", RequireAuth)
	userGroup.GET("", UserHandler)
}

// This custom Render replaces Echo's echo.Context.Render() with templ's templ.Component.Render().
func Render(ctx echo.Context, statusCode int, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(ctx.Request().Context(), buf); err != nil {
		return err
	}

	return ctx.HTML(statusCode, buf.String())
}

func HomeHandler(c echo.Context) error {
	cc := &CustomContext{c}
	return Render(c, http.StatusOK, views.Home(views.HomeData{User: cc.User()}))
}

func LoginHandler(c echo.Context) error {
	cc := &CustomContext{c}
	// Check if user is already authenticated via session
	if cc.User() != nil {
		// User is already logged in, redirect to home page
		return c.Redirect(http.StatusTemporaryRedirect, "/")
	}

	return c.Redirect(http.StatusTemporaryRedirect, cognitoHostedLoginURL())
}

func CognitoCallbackHandler(c echo.Context) error {
	logger.Info("CognitoCallbackHandler: request query parameters", "query", c.Request().URL.Query())

	code := c.QueryParam("code")
	if code == "" {
		logger.Error("CognitoCallbackHandler: no code in request")
		return echo.NewHTTPError(http.StatusBadRequest, "No authorization code provided")
	}

	// Exchange the authorization code for tokens
	tokenResponse, err := exchangeCodeForTokens(code)
	if err != nil {
		logger.Error("CognitoCallbackHandler: failed to exchange code for tokens", "error", err)
		return err
	}

	// Get user info using the access token
	userInfo, err := getUserInfo(tokenResponse.AccessToken)
	if err != nil {
		logger.Error("CognitoCallbackHandler: failed to get user info", "error", err)
		return err
	}

	logger.Info("CognitoCallbackHandler: user info", "userInfo", userInfo)

	// Store user in session
	sess, err := session.Get(sessionName, c)
	if err != nil {
		logger.Error("CognitoCallbackHandler: failed to get session", "error", err)
		return err
	}

	sess.Values[sessionUserKey] = models.User{
		ID:   userInfo.Sub,
		Name: userInfo.Name,
	}

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		logger.Error("CognitoCallbackHandler: failed to save session", "error", err)
		return err
	}

	logger.Info("CognitoCallbackHandler: completed user auth", "user", userInfo)
	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

func LogoutHandler(c echo.Context) error {
	logout(c)
	return c.Redirect(http.StatusTemporaryRedirect, cognitoHostedLogoutURL(c.Request().Host))
}

func userFromSession(c echo.Context) *models.User {
	sess, err := session.Get(sessionName, c)
	if err != nil {
		logger.Error("User: failed to get session", "error", err)
		return nil
	}

	if user, ok := sess.Values[sessionUserKey]; ok {
		var curUser = models.User{}
		if curUser, ok = user.(models.User); !ok {
			logger.Error("User: user is not the proper type", "user", user)
			return nil
		}

		return &curUser
	}

	return nil
}

func logout(c echo.Context) error {
	sess, err := session.Get(sessionName, c)
	// ignore error fetching session, as means we don't have one (most likely)
	// also ignore error saving session, but log it in case, as it's unexpected
	if err == nil {
		sess.Options.MaxAge = -1 // deletes the session
		if err := sess.Save(c.Request(), c.Response()); err != nil {
			logger.Error("logout: failed to save session", "error", err)
		}
	}

	return nil
}

func getFileSystem(useOS bool) http.FileSystem {
	if useOS {
		logger.Info("using live mode for assets")
		return http.FS(os.DirFS("assets"))
	}

	logger.Info("using embed mode for assets")
	fsys, err := fs.Sub(staticAssets, "assets")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func AdminHandler(c echo.Context) error {
	cc := &CustomContext{c}
	user := cc.User()

	// For this demo app, we just see if the user's name includes "Admin".
	// Normally you'd check a flag/role on the user record, etc.
	if !strings.Contains(user.Name, "Admin") {
		return echo.NewHTTPError(http.StatusForbidden, "You are not authorized to access this page")
	}

	return Render(c, http.StatusOK, views.Admin(*user))
}

func UserHandler(c echo.Context) error {
	cc := &CustomContext{c}
	return Render(c, http.StatusOK, views.User(*cc.User()))
}

func AddUserToContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := userFromSession(c)
		if user != nil {
			c.Set(contextUserKey, user)
		}

		return next(c)
	}
}

func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cc := &CustomContext{c}
		user := cc.User()
		if user == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "You must be logged in to access this page")
		}

		return next(c)
	}
}
