package reportinghttp

import (
	"strings"

	"github.com/gin-gonic/gin"
	reportingdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reporting/domain"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/http/ginutil"
)

// ParseQuery parses the common range contract used by operational reports.
func ParseQuery(c *gin.Context) (reportingdomain.Query, error) {
	from, err := ginutil.ParseTimeNullable(strings.TrimSpace(c.Query("from")))
	if err != nil {
		return reportingdomain.Query{}, err
	}
	to, err := ginutil.ParseTimeNullable(strings.TrimSpace(c.Query("to")))
	if err != nil {
		return reportingdomain.Query{}, err
	}
	forceRefresh, err := ginutil.ParseQueryBool(c, "force_refresh")
	if err != nil {
		return reportingdomain.Query{}, err
	}
	return reportingdomain.Query{
		Range:        strings.TrimSpace(c.DefaultQuery("range", "7d")),
		From:         from,
		To:           to,
		Timezone:     strings.TrimSpace(c.Query("tz")),
		ForceRefresh: forceRefresh,
	}, nil
}
