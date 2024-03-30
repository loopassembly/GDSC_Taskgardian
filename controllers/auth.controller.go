package controllers

import (
	"GDSC/initializers"
	"GDSC/models"
	"GDSC/utils"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt"
	"github.com/thanhpk/randstr"
	"golang.org/x/crypto/bcrypt"
)

func SignUpUser(c *fiber.Ctx) error {
	var payload *models.SignUpInput

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": err.Error()})
	}

	errors := models.ValidateStruct(payload)
	if errors != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "errors": errors})

	}

	if payload.Password != payload.PasswordConfirm {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Passwords do not match"})

	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": err.Error()})
	}

	newUser := models.User{
		Name:     payload.Name,
		Email:    strings.ToLower(payload.Email),
		Password: string(hashedPassword),
		Photo:    &payload.Photo,
	}

	result := initializers.DB.Create(&newUser)

	if result.Error != nil && strings.Contains(result.Error.Error(), "duplicate key value violates unique") {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "fail", "message": "User with that email already exists"})
	} else if result.Error != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"status": "error", "message": "Something bad happened"})
	}

	// email
	// The code below is from your initial code
	config, _ := initializers.LoadConfig(".")
	code := randstr.String(20)
	verificationCode := utils.Encode(code)
	newUser.VerificationCode = verificationCode
	initializers.DB.Save(&newUser)

	var firstName = newUser.Name
	if strings.Contains(firstName, " ") {
		firstName = strings.Split(firstName, " ")[1]
	}

	emailData := utils.EmailData{
		URL:       config.ClientOrigin + "api/auth/verifyemail/" + code,
		// URL:       "192.168.64.202:3000/api/auth/verifyemail/" + code,
		FirstName: firstName,
		Subject:   "Your account verification code",
	}

	utils.SendEmail(&newUser, &emailData, "verificationCode.html")

	message := "We sent an email with a verification code to " + newUser.Email

	// return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": fiber.Map{"user": models.FilterUserRecord(&newUser)}})
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "message": message})
}


func ManagerRegistration(c *fiber.Ctx) error {
	var payload models.Task

	// Parse request body into Task struct
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": err.Error()})
	}

	// Validate the input payload
	errors := models.ValidateStruct(payload)
	if errors != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "errors": errors})
	}

	// Create a demo task
	demoTask := models.Task{
		Title:       payload.Title,
		Description: payload.Description,
		 // Assign the task to the Manager (User ID: 1)
		Status:      payload.Status,                          // Set initial status as "To Do"
		Deadline:    time.Now().AddDate(0, 0, 7),             // Example deadline (7 days from now)
	}

	// Call the function to create the task
	result := initializers.DB.Create(&demoTask)
	if result.Error != nil {
		// Handle error
		return result.Error
	}

	// Return success message
	return c.SendString("Demo task assigned to the Manager successfully!")
}



func GetUserTasks(c *fiber.Ctx) error {
	// Extract user ID from the request query parameter
	userID := c.Params("id")

	// Fetch user from the database
	var user models.User
	if err := initializers.DB.Where("id = ?", userID).Preload("Tasks").First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "fail", "message": "User not found"})
	}

	// Return user's information along with tasks
	return c.JSON(fiber.Map{"status": "success", "data": user})
}


 func UserRegistration(c *fiber.Ctx) error {
	return c.SendString("Hello, World!")
 }



func VerifyEmail(c *fiber.Ctx) error {
	code := c.Params("verificationCode")
	verificationCode := utils.Encode(code)
	// fmt.Println(verificationCode)
	var updatedUser models.User
	result := initializers.DB.First(&updatedUser, "verification_code = ?", verificationCode)
	if result.Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Invalid verification code or user doesn't exist"})
	}

	if *updatedUser.Verified {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "fail", "message": "User already verified"})
	}

	updatedUser.VerificationCode = ""
	*updatedUser.Verified = true
	initializers.DB.Save(&updatedUser)

	// Render the email confirmation template
	if err := renderConfirmationTemplate(c); err != nil {
		log.Printf("Error rendering confirmation template: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal Server Error"})
	}

	return nil
}





func ForgotPassword(c *fiber.Ctx) error {
	var payload *models.ForgotPasswordInput

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": err.Error()})
	}

	message := "You will receive a reset email if the user with that email exists"

	var user models.User
	result := initializers.DB.First(&user, "email = ?", strings.ToLower(payload.Email))
	if result.Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Invalid email or Password"})
	}

	if !*user.Verified {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Account not verified"})
	}

	config, err := initializers.LoadConfig(".")
	if err != nil {
		log.Fatal("Could not load config", err)
	}

	// Generate Verification Code
	resetToken := randstr.String(20)

	passwordResetToken := utils.Encode(resetToken)
	user.PasswordResetToken = passwordResetToken
	user.PasswordResetAt = time.Now().Add(time.Minute * 15)
	initializers.DB.Save(&user)

	firstName := user.Name
	if strings.Contains(firstName, " ") {
		firstName = strings.Split(firstName, " ")[1]
	}

	// Send Email
	emailData := utils.EmailData{
		URL:       config.ClientOrigin + "api/auth/resetpassword/" + resetToken,
		FirstName: firstName,
		Subject:   "Your password reset token (valid for 10min)",
	}

	utils.SendEmail(&user, &emailData, "resetPassword.html")

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": message})
}

func ResetPassword(c *fiber.Ctx) error {
	var payload *models.ResetPasswordInput
	resetToken := c.Params("resetToken")

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": err.Error()})
	}

	if payload.Password != payload.PasswordConfirm {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Passwords do not match"})
	}

	hashedPassword, _ := utils.HashPassword(payload.Password)

	passwordResetToken := utils.Encode(resetToken)

	var updatedUser models.User
	result := initializers.DB.First(&updatedUser, "password_reset_token = ? AND password_reset_at > ?", passwordResetToken, time.Now())
	if result.Error != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "The reset token is invalid or has expired"})
	}

	updatedUser.Password = hashedPassword
	updatedUser.PasswordResetToken = ""
	initializers.DB.Save(&updatedUser)

	// Assuming you want to clear a token in the response
	c.ClearCookie("token")

	return c.Status(http.StatusOK).JSON(fiber.Map{"status": "success", "message": "Password data updated successfully"})
}



func renderConfirmationTemplate(c *fiber.Ctx) error {
	

	// Render the template and send the output to the client
	return c.Render("index", fiber.Map{
		"Title": "Hello, World!",
	})
	
}

func SignInUser(c *fiber.Ctx) error {
	var payload *models.SignInInput

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": err.Error()})
	}

	errors := models.ValidateStruct(payload)
	if errors != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errors)

	}

	var user models.User
	result := initializers.DB.First(&user, "email = ?", strings.ToLower(payload.Email))
	if result.Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Invalid email or Password"})
	}
	if !*user.Verified {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Email not verified"})
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(payload.Password))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "message": "Invalid email or Password"})
	}

	config, _ := initializers.LoadConfig(".")

	tokenByte := jwt.New(jwt.SigningMethodHS256)

	now := time.Now().UTC()
	claims := tokenByte.Claims.(jwt.MapClaims)

	claims["sub"] = user.ID
	claims["exp"] = now.Add(config.JwtExpiresIn).Unix()
	claims["iat"] = now.Unix()
	claims["nbf"] = now.Unix()

	tokenString, err := tokenByte.SignedString([]byte(config.JwtSecret))

	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"status": "fail", "message": fmt.Sprintf("generating JWT Token failed: %v", err)})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		MaxAge:   config.JwtMaxAge * 60,
		Secure:   false,
		HTTPOnly: true,
		Domain:   "localhost",
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "token": tokenString})
}

func LogoutUser(c *fiber.Ctx) error {
	expired := time.Now().Add(-time.Hour * 24)
	c.Cookie(&fiber.Cookie{
		Name:    "token",
		Value:   "",
		Expires: expired,
	})
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success"})
}

