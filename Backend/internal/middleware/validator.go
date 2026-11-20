package middleware

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// CustomValidator はecho.Validatorインターフェースを実装し、
// go-playground/validatorによる宣言的バリデーションを提供する。
type CustomValidator struct {
	v *validator.Validate
}

// NewCustomValidator はCustomValidatorを初期化して返す。
func NewCustomValidator() *CustomValidator {
	return &CustomValidator{v: validator.New()}
}

// Validate はstructに付与されたvalidateタグを検証する。
// バリデーションエラーは400 BadRequestとして返す。
func (cv *CustomValidator) Validate(i any) error {
	if err := cv.v.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, APIError{
			Code: "VALIDATION_ERROR",
			Msg:  validationMessage(err),
		})
	}
	return nil
}

// validationMessage は検証エラーを、画面に出せる短い文にする。
// validator の生メッセージはフィールド名の内部構造を含むので返さない。
func validationMessage(err error) string {
	var ves validator.ValidationErrors
	if !errors.As(err, &ves) {
		return "入力内容を確認してください"
	}
	emailRequired := false
	passwordRequired := false
	passwordShort := false
	for _, fe := range ves {
		switch {
		case fe.Field() == "Password" && fe.Tag() == "min":
			passwordShort = true
		case fe.Field() == "Email" && fe.Tag() == "required":
			emailRequired = true
		case fe.Field() == "Password" && fe.Tag() == "required":
			passwordRequired = true
		}
	}
	if passwordShort {
		return "password must be at least 8 characters"
	}
	if emailRequired && passwordRequired {
		return "email and password are required"
	}
	if emailRequired {
		return "email is required"
	}
	if passwordRequired {
		return "email and password are required"
	}
	return "入力内容を確認してください"
}
