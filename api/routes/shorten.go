package routes

import(
  "time"
  "github.com/BREACH1247/url-shortner/databases"
  "github.com/BREACH1247/url-shortner/helpers"
  "os"
  "strconv"
  "github.com/go-redis/redis/v8"
  "github.com/gofiber/fiber/v2"
  "github.com/asaskevich/govalidator"
  "github.com/google/uuid"
  "fmt"
)

type request struct {
	URL       string `json:"url"`
	CustomURl string `json:"short"`
	Expiry    time.Duration  `json:"expires"`
}

type response struct {
	URL string `json:"url"`
	CustomURl string `json:"short"`
	Expiry  time.Duration `json:"expires"`
	XRateRemaining  int   `json:"rate_limit"`
	XRateLimitReset time.Duration `json:"rate_limit_reset"`
}


func ShortenURL(c *fiber.Ctx) error {
  body := new(request)

  if err := c.BodyParser(&body); err != nil {
	fmt.Println("Error while parsing data")
	return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{"error":"cannot parse json"})
  }
 // implement rate limiting
 r2 := database.CreateClient(1)
 defer r2.Close()
 val, err :=r2.Get(database.Ctx,c.IP()).Result()
 if err == redis.Nil{
	_ = r2.Set(database.Ctx,c.IP(),os.Getenv("API_QUOTA"),30*60*time.Second).Err()
 } else {
	valInt,_ := strconv.Atoi(val)
	if valInt <= 0 {
		limit,_ := r2.TTL(database.Ctx,c.IP()).Result()
		return c.Status(fiber.StatusServiceUnavailable).JSON(&fiber.Map{"error":"rate limit exceeded","rate_limit_rest": limit/time.Nanosecond/time.Minute})
	}
 }


 //valid url
 if !govalidator.IsURL(body.URL){
	return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{"error":"invalid url",})
 }

 // domain error 
 if !helpers.RemoveDomainError(body.URL){
	return c.Status(fiber.StatusServiceUnavailable).JSON(&fiber.Map{"error" : "invalid domain name"})
 }

 // enforces SSL
   body.URL = helpers.EnforceHTTP(body.URL)

   var id string

   if body.CustomURl == ""{
	 id = uuid.New().String()[:6]
   } else{
	id = body.CustomURl
   }

   r := database.CreateClient(0)
   defer r.Close()

   val,_  = r.Get(database.Ctx,id).Result()
   if val != ""{
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": "URL custom short already in use",
	})
   }

   if body.Expiry == 0{
	body.Expiry = 24
   }

   err = r.Set(database.Ctx,id,body.URL,body.Expiry*3600*time.Second).Err()

   if err != nil {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": "Unable to connect to the server",
	})
   }
   

   resp := response{
	URL :  body.URL,
	CustomURl : "",
	Expiry: body.Expiry,
	XRateRemaining: 10,
	XRateLimitReset: 30,
   }


   r2.Decr(database.Ctx,c.IP())

   val,_ = r2.Get(database.Ctx, c.IP()).Result()
   resp.XRateRemaining,_ = strconv.Atoi(val)

   ttl,_ := r2.TTL(database.Ctx, c.IP()).Result()
   resp.XRateLimitReset = ttl/time.Nanosecond/time.Minute

   resp.CustomURl = os.Getenv("DOMAIN") + "/" + id

   return c.Status(fiber.StatusOK).JSON(resp)

}