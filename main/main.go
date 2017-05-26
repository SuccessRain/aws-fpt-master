package main

import (
	"os"
	"bufio"
	"strings"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/lexruntimeservice"
	"github.com/aws/aws-sdk-go/service/lexmodelbuildingservice"
)

var target string
var inputFP string
var accessKey string
var secretKey string
var botName string

func AddSharedFlags(fs *flag.FlagSet) {
	fs.StringVar(&target, "t", "", "required, intent or entity")
	fs.StringVar(&inputFP, "i", "", "required, path to the input file")
	fs.StringVar(&accessKey, "a", "", "required, AWS access key")
	fs.StringVar(&secretKey, "s", "", "required, secret key")
	fs.StringVar(&botName, "b", "", "required, bot name")
}

type ObjectFile struct {
	Intent string
	Name   string
}

func ReadIntentsFromFile(inputFP string) ([]ObjectFile, error) {
	input, err := os.Open(inputFP)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	var context []ObjectFile
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var con ObjectFile
		text := strings.Replace(scanner.Text(), `"`, ``, -1)
		tokens := strings.SplitN(text, ",", 2)
		con.Intent, con.Name = strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1])
		context = append(context, con)
	}

	return context, nil
}

func testData(sess *session.Session, botName string, textInput string, result string) bool {
	svc := lexruntimeservice.New(sess)

	//Test data CongVV
	params := &lexruntimeservice.PostTextInput{
		BotAlias:  aws.String(botName), // Required
		BotName:   aws.String(botName),   // Required
		InputText: aws.String(textInput), // Required
		UserId:    aws.String("congvv"),  // Required
	}

	resp, err := svc.PostText(params)

	if err != nil {
		fmt.Println(err)
		return false
	}

	if resp.IntentName == nil{
		return false
	} else if (strings.Compare(strings.TrimSpace(strings.ToLower(result)),
		strings.TrimSpace(strings.ToLower(*resp.IntentName))) == 0) {
		return true
	} else {
		return false
	}
}

type ObjectMutilValue struct {
	intentName string
	values     []string
}

func recycleInentName(objs []ObjectFile) []ObjectMutilValue {

	var omvs []ObjectMutilValue

	data := make(map[string][]string)
	for _, a := range objs {
		samples := data[a.Intent]
		samples = append(samples, a.Name)
		data[a.Intent] = samples
	}

	for key, value := range data {

		var omv ObjectMutilValue
		omv.intentName = key
		omv.values = value
		omvs = append(omvs, omv)
	}

	return omvs
}

func convertArrStringToPointArrString(values []string) []*string {
	var sample []*string
	for i := 0; i < len(values); i++{
		if len(values[i]) > 200 {
			values[i] = strings.Replace(values[i], "(", "", -1)
			values[i] = strings.Replace(values[i], ")", "", -1)
			values[i] = strings.Replace(values[i], "?", "", -1)
			values[i] = strings.Replace(values[i], "@", "", -1)
			values[i] = string(values[i][0:199])
		}
		sample = append(sample, &values[i])
	}
	return sample
}

func convertArrIntentToPointArrIntent(multi *[]ObjectMutilValue) []*lexmodelbuildingservice.Intent {
	var intents []*lexmodelbuildingservice.Intent
	var intent *lexmodelbuildingservice.Intent
	for _, a := range *multi {
		intent = &lexmodelbuildingservice.Intent{
			IntentName:    aws.String(a.intentName),
			IntentVersion: aws.String("$LATEST"),
		}
		intents = append(intents, intent)
	}
	return intents
}

func trainData(sess *session.Session, obj []ObjectFile, botName string) bool {
	svc := lexmodelbuildingservice.New(sess)
	multi := recycleInentName(obj)
	i := 0
	var intentsString []string
	for _, a := range multi {
		sam := convertArrStringToPointArrString(a.values)
		st := putIntent(svc, a.intentName, sam, &i)
		if strings.Compare(st,"nil") != 0 {
			intentsString = append(intentsString, st)
		}
	}

	//putBot(svc, botName, convertArrIntentStringToPointArrIntent(&intentsString))
	putBot(svc, botName, convertArrIntentToPointArrIntent(&multi))

	return true
}

func putBot(svc *lexmodelbuildingservice.LexModelBuildingService, botName string, intentNames []*lexmodelbuildingservice.Intent) {

	var intentNames2 []*lexmodelbuildingservice.Intent
	for _,a := range intentNames{
		fmt.Print("---INTENTNAME:\t"); fmt.Println(len(*a.IntentName))
		if CheckExitsIntent(svc, *a.IntentName) {
			intentNames2 = append(intentNames2, a)
		}else{
			fmt.Print("---NOT TRUEEEE:\t"); fmt.Println(len(*a.IntentName))
		}
	}
	fmt.Print("---LENGTHHHHHHHHHH:\t"); fmt.Println(len(intentNames2))
	params := &lexmodelbuildingservice.PutBotInput{
		ChildDirected: aws.Bool(false),      // Required
		Locale:        aws.String("en-US"), // Required
		Name:          aws.String(botName), // Required
		AbortStatement: &lexmodelbuildingservice.Statement{
			Messages: []*lexmodelbuildingservice.Message{ // Required
				{ // Required
					Content:     aws.String("ContentString"), // Required
					ContentType: aws.String("PlainText"),   // Required
				},
				// More values...
			},
			ResponseCard: aws.String("ResponseCard1"),
		},
		ClarificationPrompt: &lexmodelbuildingservice.Prompt{
			MaxAttempts: aws.Int64(1), // Required
			Messages: []*lexmodelbuildingservice.Message{ // Required
				{ // Required
					Content:     aws.String("Sorry. Can you repeat it"), // Required
					ContentType: aws.String("PlainText"),   // Required
				},
				{ // Required
					Content:     aws.String("Sorry"), // Required
					ContentType: aws.String("PlainText"),   // Required
				},
			},
			ResponseCard: aws.String("ResponseCard2"),
		},
		Intents:       intentNames2,
	}
	resp, err := svc.PutBot(params)

	if err != nil {
		fmt.Println(err.Error())
		return
	}else{
		fmt.Print("Put bot ");
		fmt.Print(*resp.Name);
		fmt.Println(" success!")
	}
}

func CheckExitsIntent(svc *lexmodelbuildingservice.LexModelBuildingService, intentName string) bool {
	params := &lexmodelbuildingservice.GetIntentInput{
		Name:    aws.String(intentName), // Required
		Version: aws.String("$LATEST"),    // Required
	}
	resp, err := svc.GetIntent(params)
	if err == nil {
		fmt.Print("OKKKKKKK:\t");fmt.Println(*resp.Name)
		return true
	}else{
		fmt.Print("ERRROOORR:\t");fmt.Println(err)
	}
	return false;
}

func convertArrIntentStringToPointArrIntent(multi *[]string) []*lexmodelbuildingservice.Intent {
	var intents []*lexmodelbuildingservice.Intent
	var intent *lexmodelbuildingservice.Intent
	for _, a := range *multi {
		intent = &lexmodelbuildingservice.Intent{
			IntentName:    aws.String(a),
			IntentVersion: aws.String("$LATEST"),
		}
		intents = append(intents, intent)
	}
	return intents
}

func putIntent(svc *lexmodelbuildingservice.LexModelBuildingService, intentName string,
	sample []*string, i *int) string {
	var result string = "nil"
	params := &lexmodelbuildingservice.PutIntentInput{
		Name:             aws.String(intentName),
		SampleUtterances: sample,
	}
	resp, err := svc.PutIntent(params)

	if err != nil {
		fmt.Println(err.Error())
	} else {
		*i = *i + 1
		fmt.Print(*i)
		fmt.Print(". Put intent ")
		fmt.Print(*resp.Name)
		fmt.Println(" success!")
		result = intentName
	}
	return result
}

func main() {
	/*
	creds := credentials.NewStaticCredentials("AKIAJ6M366QNRZGYTPYQ", "nODExxtAjdjs2NLJdZbsc4EMCCEEEFFYd/VA1NYR", "")
	sess := session.New(&aws.Config{Region: aws.String("us-east-1"), Credentials: creds })
	svc := lexmodelbuildingservice.New(sess)
	CheckExitsIntent(svc, "DatacardAvailConfirm")
	*/


	trainCmd := flag.NewFlagSet("train", flag.ExitOnError)
	AddSharedFlags(trainCmd)

	testCmd := flag.NewFlagSet("test", flag.ExitOnError)
	AddSharedFlags(testCmd)

	if len(os.Args) < 2 {
		fmt.Println("Error: Input is not enough")
		fmt.Println(helpMessage)
		os.Exit(1)
	}

	command := os.Args[1]
	if command == "train" {
		trainCmd.Parse(os.Args[2:])
	} else if command == "test" {
		testCmd.Parse(os.Args[2:])
	} else if command == "help" {
		fmt.Println(helpMessage)
		os.Exit(0)
	}

	if target != "intent" && target != "entity" {
		fmt.Println("Error: You must choose intent or entity")
		fmt.Println(helpMessage)
		os.Exit(1)
	}

	if inputFP == "" {
		fmt.Println("Error: Input file is required but empty")
		fmt.Println(helpMessage)
		os.Exit(1)
	}

	if accessKey == "" {
		fmt.Println("Error: AccessKey id is required")
		fmt.Println(helpMessage)
		os.Exit(1)
	}

	if secretKey == "" {
		fmt.Println("Error: SecretKey is required")
		fmt.Println(helpMessage)
		os.Exit(1)
	}

	if botName == "" {
		fmt.Println("Error: Botname is required")
		fmt.Println(helpMessage)
		os.Exit(1)
	}

	//ak := "AKIAJ6M366QNRZGYTPYQ"
	//sk := "nODExxtAjdjs2NLJdZbsc4EMCCEEEFFYd/VA1NYR"

	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	sess := session.New(&aws.Config{Region: aws.String("us-east-1"), Credentials: creds })
	obj, err := ReadIntentsFromFile(inputFP)
	if err != nil {
		fmt.Println(err)
	}

	if trainCmd.Parsed() {
		if target == "intent" {
			trainData(sess, obj, botName)
		} else if target == "entity" {
			fmt.Println("Ahihi! Chua co cho entity")
		}
	} else if testCmd.Parsed() {
		if target == "intent" {
			var count int = 0
			i := 1
			for _,a := range obj{
				b := testData(sess, botName, a.Name, a.Intent)
				if b {
					fmt.Print(i); fmt.Println(". Correct")
					count ++;
				}else {
					fmt.Print(i); fmt.Println(". Incorrect")
				}
				i++
			}
			fmt.Printf("Success: %f \n", float64(count) / float64(len(obj)) * 100)
		} else if target == "entity" {
			fmt.Println("Ahihi! Chua co cho entity")
		}
	}
}

var helpMessage = ``
