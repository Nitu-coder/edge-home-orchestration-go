// +build secure

/*******************************************************************************
 * Copyright 2019 Samsung Electronics All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 *******************************************************************************/

// Package main provides REST interface for edge-orchestration
package main

import (
	"errors"
	"flag"
	"log"
	"time"

	"common/logmgr"

	configuremgr "controller/configuremgr/container"
	"controller/discoverymgr"
	"controller/scoringmgr"
	"controller/servicemgr"
	executor "controller/servicemgr/executor/containerexecutor"

	"orchestrationapi"

	"restinterface/cipher/dummy"
	"restinterface/client/restclient"
	"restinterface/externalhandler"
	"restinterface/internalhandler"
	"restinterface/route"

	"db/bolt/wrapper"
)

const logPrefix = "interface"

// Handle Platform Dependencies
const (
	platform      = "docker"
	executionType = "container"

	edgeDir = "/var/edge-orchestration"

	logPath             = edgeDir + "/log"
	configPath          = edgeDir + "/apps"
	dbPath              = edgeDir + "/data/db"
	certificateFilePath = edgeDir + "/data/cert"

	cipherKeyFilePath = edgeDir + "/user/orchestration_userID.txt"
	deviceIDFilePath  = edgeDir + "/device/orchestration_deviceID.txt"
)

var (
	flagVersion                  bool
	commitID, version, buildTime string
	buildTags                    string
)

func main() {
	if err := orchestrationInit(); err != nil {
		log.Fatalf("[%s] Orchestaration initalize fail : %s", logPrefix, err.Error())
	}

	for {
		time.Sleep(1000)
	}
}

// orchestrationInit runs orchestration service and discovers other orchestration services in other devices
func orchestrationInit() error {
	flag.BoolVar(&flagVersion, "v", false, "if true, print version and exit")
	flag.BoolVar(&flagVersion, "version", false, "if true, print version and exit")
	flag.Parse()

	logmgr.Init(logPath)
	log.Printf("[%s] OrchestrationInit", logPrefix)
	log.Println(">>> commitID  : ", commitID)
	log.Println(">>> version   : ", version)
	log.Println(">>> buildTime : ", buildTime)
	log.Println(">>> buildTags : ", buildTags)
	wrapper.SetBoltDBPath(dbPath)

	restIns := restclient.GetRestClient()
	restIns.SetCipher(dummy.GetCipher(cipherKeyFilePath))

	servicemgr.GetInstance().SetClient(restIns)

	builder := orchestrationapi.OrchestrationBuilder{}
	builder.SetWatcher(configuremgr.GetInstance(configPath))
	builder.SetDiscovery(discoverymgr.GetInstance())
	builder.SetScoring(scoringmgr.GetInstance())
	builder.SetService(servicemgr.GetInstance())
	builder.SetExecutor(executor.GetInstance())
	builder.SetClient(restIns)

	orcheEngine := builder.Build()
	if orcheEngine == nil {
		log.Fatalf("[%s] Orchestaration initalize fail", logPrefix)
		return errors.New("fail to init orchestration")
	}

	orcheEngine.Start(deviceIDFilePath, platform, executionType)

	restEdgeRouter := route.NewRestRouterWithCerti(certificateFilePath)

	internalapi, err := orchestrationapi.GetInternalAPI()
	if err != nil {
		log.Fatalf("[%s] Orchestaration internal api : %s", logPrefix, err.Error())
	}
	ihandle := internalhandler.GetHandler()
	ihandle.SetOrchestrationAPI(internalapi)
	ihandle.SetCipher(dummy.GetCipher(cipherKeyFilePath))
	ihandle.SetCertificateFilePath(certificateFilePath)
	restEdgeRouter.Add(ihandle)

	// external rest api
	externalapi, err := orchestrationapi.GetExternalAPI()
	if err != nil {
		log.Fatalf("[%s] Orchestaration external api : %s", logPrefix, err.Error())
	}
	ehandle := externalhandler.GetHandler()
	ehandle.SetOrchestrationAPI(externalapi)
	ehandle.SetCipher(dummy.GetCipher(cipherKeyFilePath))
	restEdgeRouter.Add(ehandle)

	restEdgeRouter.Start()

	log.Println(logPrefix, "orchestration init done")

	// TODO remove this line
	// this line is for wait to initialize the mDNS server.
	time.Sleep(time.Second * 2)

	return nil
}
