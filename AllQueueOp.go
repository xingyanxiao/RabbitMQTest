package RabbitMQTest

import (
	"time"
	"log"
	"strconv"
	"fmt"
)

func AllQueueOp(key string,config map[string]string,resultDataChan chan *Result){
	ResultData := NewResult()
	defer func() {
		resultDataChan <- ResultData
	}()

	AmqpUri := config["AmqpUri"]
	HttpUri := config["HttpUri"]
	AmqpAdmin := config["AmqpAdmin"]
	AmqpPwd := config["AmqpPwd"]

	var qList *[]queueInfo
	if _,ok:=config["QueueList"];ok{
		qList = GetQueuesByConfig(config["QueueList"])
	}else{
		qList = GetQueuesByUrl(HttpUri,AmqpAdmin,AmqpPwd)
	}

	AllStartTime := time.Now().UnixNano() / 1e6

	OverCount := 0
	NeedWaitCount := 0
	WriteNeedWaitCount,ConsumeNeedWaitCount:=0,0
	ResultChan := make(chan *Result,10000)

	log.Println(key,"AllQueueOp start",AllStartTime)

	var ContinueCount int
	var ContinueCountSleepTime int
	if _,ok:= config["ContinueCount"];ok{
		ContinueCount = GetIntDefault(config["ContinueCount"],0)
	}

	if _,ok:= config["ContinueCountSleepTime"];ok{
		ContinueCountSleepTime = GetIntDefault(config["ContinueCountSleepTime"],0)
	}


	for keyI,qInfo := range *qList{
		if keyI >0 && ContinueCount > 0 && keyI % ContinueCount == 0{
			time.Sleep(time.Duration(ContinueCountSleepTime) * time.Second)
		}
		m := make(map[string]string)
		if qInfo.Vhost == "/"{
			m["Uri"] = "amqp://"+AmqpAdmin+":"+AmqpPwd+"@"+AmqpUri+"/"
		}else{
			m["Uri"] = "amqp://"+AmqpAdmin+":"+AmqpPwd+"@"+AmqpUri+"/"+qInfo.Vhost
		}

		keyString := key+"-"+config["Method"]+strconv.Itoa(keyI)

		switch config["Method"] {
		case "all_write":
			m["ConnectCount"] = config["ConnectCount"]
			m["DeliveryMode"] = config["DeliveryMode"]
			m["DataSize"] = config["DataSize"]
			m["ChannelCount"] = config["ChannelCount"]
			m["ChanneWriteCount"] = config["ChanneWriteCount"]
			m["WaitConfirm"] = config["WaitConfirm"]
			m["WriteTimeOut"] = config["WriteTimeOut"]
			if _,ok:=config["ExchangeName"];ok{
				m["ExchangeName"] = config["ExchangeName"]
			}else{
				m["ExchangeName"] = ""
			}
			m["RoutingKey"] = qInfo.Queue
			go SingleSend(keyString,m,ResultChan)
			NeedWaitCount++
			WriteNeedWaitCount++
			break
		case "all_consume":
			m["ConnectCount"] = config["ConnectCount"]
			m["QueueName"] = qInfo.Queue
			m["ConsumeTimeOut"] = config["ConsumeTimeOut"]
			if _,ok:=config["ConsumeCount"];ok{
				m["ConsumeCount"] = config["ConsumeCount"]
			}else{
				m["ConsumeCount"] = "0"
			}
			if _,ok:=config["AutoAck"];ok{
				m["AutoAck"] = config["AutoAck"]
			}else{
				m["AutoAck"] = "0"
			}

			NeedWaitCount++
			ConsumeNeedWaitCount++
			go SingleConsume(keyString,m,ResultChan)
			break
		default:
			m2 := make(map[string]string)
			NeedWaitCount += 2
			m2["Uri"] = m["Uri"]
			m2["QueueName"] = qInfo.Queue
			m2["ConnectCount"] = config["CosumeConnectCount"]
			m2["ConsumeTimeOut"] = config["ConsumeTimeOut"]
			if _,ok:=config["AutoAck"];ok{
				m2["AutoAck"] = config["AutoAck"]
			}else{
				m2["AutoAck"] = "0"
			}
			if _,ok:=config["ConsumeCount"];ok{
				m2["ConsumeCount"] = config["ConsumeCount"]
			}else{
				m2["ConsumeCount"] = "0"
			}
			go SingleConsume(keyString,m2,ResultChan)
			ConsumeNeedWaitCount++

			m["ConnectCount"] = config["WriteConnectCount"]
			m["DeliveryMode"] = config["DeliveryMode"]
			m["DataSize"] = config["DataSize"]
			m["ChannelCount"] = config["ChannelCount"]
			m["ChanneWriteCount"] = config["ChanneWriteCount"]
			m["WaitConfirm"] = config["WaitConfirm"]
			m["WriteTimeOut"] = config["WriteTimeOut"]
			if _,ok:=config["ExchangeName"];ok{
				m["ExchangeName"] = config["ExchangeName"]
			}else{
				m["ExchangeName"] = ""
			}
			m["RoutingKey"] = qInfo.Queue
			go SingleSend(keyString,m,ResultChan)
			WriteNeedWaitCount++
			break
		}
	}

	if NeedWaitCount == 0{
		return
	}
	var QPSDisplay = func(){
		AllEndTime := time.Now().UnixNano() / 1e6
		fmt.Println(" ")
		UseTime := float64(AllEndTime-AllStartTime)
		log.Println(key,"AllQueueOp ",AllEndTime," had use time(ms):",UseTime)
		fmt.Println("ConnectSuccess:",ResultData.ConnectSuccess)
		fmt.Println("ConnectFail:",ResultData.ConnectFail)
		fmt.Println("ChannelSuccess:",ResultData.ChannelSuccess)
		fmt.Println("ChanneFail:",ResultData.ChanneFail)
		fmt.Println("WriteSuccess:",ResultData.WriteSuccess)
		fmt.Println("WriteFail:",ResultData.WriteFail)
		fmt.Println("CosumeSuccess:",ResultData.CosumeSuccess)
		fmt.Println("Write QPS:",float64(ResultData.WriteSuccess)/UseTime*1000)
		fmt.Println("Consume QPS:",float64(ResultData.CosumeSuccess)/UseTime*1000)
	}

	OverConsumeNeedWaitCount,OverWriteNeedWaitCount := 0,0
	loop:
	for{
		select {
		case data := <-ResultChan:
			ResultData.ConnectSuccess += data.ConnectSuccess
			ResultData.ConnectFail += data.ConnectFail
			ResultData.ChannelSuccess += data.ChannelSuccess
			ResultData.ChanneFail += data.ChanneFail
			ResultData.WriteSuccess += data.WriteSuccess
			ResultData.WriteFail += data.WriteFail
			ResultData.CosumeSuccess += data.CosumeSuccess
			OverCount++
			if OverCount >= NeedWaitCount {
				break loop
			}
			if data.Type == 1{
				OverWriteNeedWaitCount++
				if OverWriteNeedWaitCount >= WriteNeedWaitCount{
					QPSDisplay()
				}
			}else{
				OverConsumeNeedWaitCount++
				if OverConsumeNeedWaitCount >= ConsumeNeedWaitCount{
					QPSDisplay()
				}
			}
		case <-time.After(10 * time.Second):
			QPSDisplay()
			break
		}
	}
	//QPSDisplay()
}
