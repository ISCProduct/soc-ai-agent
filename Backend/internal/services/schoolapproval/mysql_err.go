package schoolapproval

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

// isDuplicateEntryErr はMySQLの一意制約違反(Error 1062)かどうか。
func isDuplicateEntryErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
