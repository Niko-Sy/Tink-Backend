package authentic

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Nickname string `json:"nickname" binding:"required,min=1,max=50"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
	Bio      string `json:"bio"`
}

type RegisterResponse struct {
	UserId        string    `json:"userId"`
	Username      string    `json:"username"`
	Nickname      string    `json:"nickname"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	Avatar        string    `json:"avatar"`
	Bio           string    `json:"bio"`
	OnlineStatus  string    `json:"onlineStatus"`
	AccountStatus string    `json:"accountStatus"`
	SystemRole    string    `json:"systemRole"`
	RegisterTime  time.Time `json:"registerTime"`
}

func HandleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 获取数据库查询对象
	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取数据库连接失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查用户名是否已存在
	usernameExists, err := queries.CheckUsernameExists(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "检查用户名失败",
			"error":   err.Error(),
		})
		return
	}
	if usernameExists {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "用户名已存在",
		})
		return
	}

	// 检查邮箱是否已存在
	emailExists, err := queries.CheckEmailExists(c.Request.Context(), sql.NullString{
		String: req.Email,
		Valid:  true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "检查邮箱失败",
			"error":   err.Error(),
		})
		return
	}
	if emailExists {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "邮箱已被注册",
		})
		return
	}

	// 如果提供了手机号，检查手机号是否已存在
	if req.Phone != "" {
		phoneExists, err := queries.CheckPhoneExists(c.Request.Context(), sql.NullString{
			String: req.Phone,
			Valid:  true,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "检查手机号失败",
				"error":   err.Error(),
			})
			return
		}
		if phoneExists {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": "手机号已被注册",
			})
			return
		}
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
			"error":   err.Error(),
		})
		return
	}

	// 创建用户参数
	createParams := sqlcdb.CreateUserParams{
		Username:       req.Username,
		HashedPassword: string(hashedPassword),
		Nickname: sql.NullString{
			String: req.Nickname,
			Valid:  req.Nickname != "",
		},
		PhoneNumber: sql.NullString{
			String: req.Phone,
			Valid:  req.Phone != "",
		},
		Email: sql.NullString{
			String: req.Email,
			Valid:  req.Email != "",
		},
		AvatarUrl: sql.NullString{
			String: req.Avatar,
			Valid:  req.Avatar != "",
		},
		Bio: sql.NullString{
			String: req.Bio,
			Valid:  req.Bio != "",
		},
	}

	// 创建用户
	user, err := queries.CreateUser(c.Request.Context(), createParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建用户失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应
	response := RegisterResponse{
		UserId:        user.UserID,
		Username:      user.Username,
		Nickname:      user.Nickname.String,
		Email:         user.Email.String,
		Phone:         user.PhoneNumber.String,
		Avatar:        user.AvatarUrl.String,
		Bio:           user.Bio.String,
		OnlineStatus:  string(user.OnlineStatus.UserOnlineStatus),
		AccountStatus: string(user.AccountStatus.UserAccountStatus),
		SystemRole:    string(user.SystemRole.UserSystemRole),
		RegisterTime:  user.RegisteredAt,
	}

	// 生成JWT token
	token, err := middleware.GenerateToken(user.UserID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成Token失败",
			"error":   err.Error(),
		})
		return
	}

	// 生成Refresh Token
	refreshToken, err := middleware.GenerateRefreshToken(user.UserID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成RefreshToken失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "注册成功",
		"data": gin.H{
			"user":         response,
			"token":        token,
			"refreshToken": refreshToken,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
