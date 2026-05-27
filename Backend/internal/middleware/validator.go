package middleware

import (
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
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}
