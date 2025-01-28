package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"time"
)

type ToDoItem struct {
	Id          int        `json:"id" gorm:"column:id;"`
	Title       string     `json:"title" gorm:"column:title;"`
	Description string     `json:"description" gorm:"column:description;"`
	Status      string     `json:"status" gorm:"column:status;"`
	CreatedAt   *time.Time `json:"created_at" gorm:"column:created_at;"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" gorm:"column:updated_at;"` // omitempty - value will be omitted if it is nil
}

type ToDoItemCreate struct {
	Id          int    `json:"-" gorm:"column:id;"`
	Title       string `json:"title" binding:"required" gorm:"column:title;"`
	Description string `json:"description" gorm:"column:description;"`
	Status      string `json:"status" gorm:"column:status;"`
}

type ToDoItemUpdate struct {
	Title       *string `json:"title" gorm:"column:title;"`
	Description *string `json:"description" gorm:"column:description;"`
	Status      *string `json:"status" gorm:"column:status;"` // using pointer to differentiate between nil and empty string
}

type Paging struct {
	Page  int   `json:"page" form:"page"`
	Limit int   `json:"limit" form:"limit"`
	Total int64 `json:"total" form:"-"`
}

func ConnectDB() *gorm.DB {
	dsn := os.Getenv("GO_DB_CONN_STR_MYSQL")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&ToDoItem{}); err != nil {
		log.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}

func (ToDoItemCreate) TableName() string {
	return "to_do_items"
}

func (ToDoItem) TableName() string {
	return "to_do_items"
}

func (ToDoItemUpdate) TableName() string {
	return "to_do_items"
}

func (p *Paging) Process() {
	if p.Page == 0 {
		p.Page = 1
	}

	if p.Limit == 0 || p.Limit >= 100 {
		p.Limit = 10
	}
}

func CreateItem(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var itemCreate ToDoItemCreate
		if err := c.ShouldBindJSON(&itemCreate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Create(&itemCreate).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Item created successfully",
			"data":    itemCreate.Id,
		})
	}
}

func GetItemById(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var item ToDoItem
		if err := db.First(&item, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": item})
	}
}

func ToDoItemUpdateById(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var itemUpdate ToDoItemUpdate
		if err := c.ShouldBindJSON(&itemUpdate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var item ToDoItem
		if err := db.First(&item, c.Param("id")).Updates(&itemUpdate).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Item updated successfully"})
	}
}

func DeleteItemById(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var item ToDoItem
		if err := db.First(&item, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
			return
		}

		// Soft delete
		if err := db.Model(&item).Updates(map[string]interface{}{"status": "deleted"}).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Item deleted successfully"})
	}
}

func GetListItems(db *gorm.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		var paging Paging

		if err := c.ShouldBindQuery(&paging); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		paging.Process()

		if err := db.Table(ToDoItem{}.TableName()).Count(&paging.Total).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var items []ToDoItem

		if err := db.Where("status <> ?", "deleted").Order("id desc").Limit(paging.Limit).Offset((paging.Page - 1) * paging.Limit).Find(&items).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": items, "paging": paging})
	}
}

func main() {
	db := ConnectDB()

	r := gin.Default()

	apiV1 := r.Group("/api/v1")
	{
		items := apiV1.Group("/items")
		{
			items.GET("", GetListItems(db))
			items.POST("", CreateItem(db))
			items.GET("/:id", GetItemById(db))
			items.PUT("/:id", ToDoItemUpdateById(db))
			items.DELETE("/:id", DeleteItemById(db))
		}
	}

	if err := r.Run(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
