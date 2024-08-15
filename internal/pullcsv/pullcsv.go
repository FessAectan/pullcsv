package pullcsv

import (
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"pullcsv/internal/logger"
	"pullcsv/internal/prom"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/go-co-op/gocron"
	"pullcsv/internal/helpers"
)

func Pullcsv(metrics *prom.Metrics) {

	if _, err := os.Stat("/usr/bin/rsync"); err != nil {
		logger.Fatal(err.Error())
	}

	dFrom, dTo, standName, podName, err := helpers.PrepareEnv()
	if err != nil {
		logger.Fatal(err.Error())
	}

	var exFNfullLocalPath,
		exFNfullRemotePath []string

	s := gocron.NewScheduler(time.UTC)

	for i, _ := range dFrom {
		exFN, err := helpers.GetExludeFileName(dFrom[i], dTo[i])
		if err != nil {
			logger.Fatal(err.Error())
		}

		re := regexp.MustCompile(`rsync.+@[a-zA-z0-9-_]+/`)
		exFNfullRemotePath = append(exFNfullRemotePath, re.FindString(dFrom[i])+"pullcsv-exclude-files/"+exFN)
		exFNfullLocalPath = append(exFNfullLocalPath, "/tmp/"+exFN)

		_, err = s.Cron(os.Getenv("DOWNLOAD_CRON")).SingletonMode().Do(func(j int) {
			var dFromStr string
			dFromStr = helpers.EnvReplacement(dFrom[j])
			if helpers.Rsync("/usr/bin/rsync "+exFNfullRemotePath[j]+" "+exFNfullLocalPath[j]) != 0 {
				os.Create(exFNfullLocalPath[j])
			}

			tmpDirDownloadTo, err := os.MkdirTemp("/tmp/", strings.ReplaceAll(dTo[j], "/", "_"))
			if err != nil {
				logger.Fatal(err.Error())
			}

			logger.Info("Start downloading files from " + dFromStr + " to " + tmpDirDownloadTo)

			rsyncCSVstartTime := time.Now().Unix()
			rsyncExitCode := helpers.Rsync("/usr/bin/rsync -azq --partial --exclude-from=" + exFNfullLocalPath[j] + " " + dFromStr + " " + tmpDirDownloadTo)
			rsyncCSVstopTime := time.Now().Unix()
			metrics.RsyncCSVExitCode.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(rsyncExitCode))
			metrics.RsyncCSVStartTime.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(rsyncCSVstartTime))
			metrics.RsyncCSVStopTime.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(rsyncCSVstopTime))

			if rsyncExitCode != 0 {
				logger.Warn("A problem with rsync (from " + dFromStr + " to " + tmpDirDownloadTo + "), the exit code: " + strconv.Itoa(rsyncExitCode) + ", it means: " + helpers.GetRsyncExitCodeMeaning(rsyncExitCode))
			} else if rsyncExitCode == 0 {
				if err := helpers.LogEveryFileAndMoveIt(dTo[j], tmpDirDownloadTo); err != nil {
					logger.Warn("Something wrong with moving downloaded files from temp location, the error: " + err.Error())
				}
				//work with archives
				if err := helpers.WorkWithArchives(dTo[j]); err != nil {
					logger.Warn(err.Error())
				}
				newestFileTimestamp, oldestFileTimestamp, countFiles := helpers.GetOldestNewestCountFiles(dTo[j])
				metrics.MaxModifiedFileLifetime.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(oldestFileTimestamp))
				metrics.MinModifiedFileLifetime.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(newestFileTimestamp))
				metrics.CountFiles.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(countFiles))

				saveToExFN, err := helpers.SaveExcludeFile(dTo[j], exFNfullLocalPath[j])
				if err != nil {
					logger.Warn("Something was wrong with saving exclude file " + exFNfullLocalPath[j] + ", the error: " + err.Error())
				} else {
					script.Slice(saveToExFN).WriteFile(exFNfullLocalPath[j])
					//truncate excludeFiles
					helpers.TruncateExcludeFile(exFNfullLocalPath[j], j, 20000, 9437184)

					rsyncEXfileStartTime := time.Now().Unix()
					rsyncExitCode = helpers.Rsync("/usr/bin/rsync " + exFNfullLocalPath[j] + " " + exFNfullRemotePath[j])
					rsyncEXfileStopTime := time.Now().Unix()
					if rsyncExitCode != 0 {
						logger.Warn("A problem with uploading exclude file to the server ( " + exFNfullLocalPath[j] + " to " + exFNfullRemotePath[j] + "), the exit code: " + strconv.Itoa(rsyncExitCode) + ", it means: " + helpers.GetRsyncExitCodeMeaning(rsyncExitCode))
					}
					metrics.RsyncEXfileStartTime.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(rsyncEXfileStartTime))
					metrics.RsyncEXfileExitCode.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(rsyncExitCode))
					metrics.RsyncEXfileStopTime.With(prometheus.Labels{"path": dTo[j], "stand_name": standName, "pod_name": podName}).Set(float64(rsyncEXfileStopTime))
				}
			}
			logger.Info("Stop downloading files from " + dFromStr + " to " + tmpDirDownloadTo)
			if err := os.RemoveAll(tmpDirDownloadTo); err != nil {
				logger.Warn("Couldn't delete tmp dir " + tmpDirDownloadTo + ", the error: " + err.Error())
			}
		}, i)
		if err != nil {
			logger.Fatal("Something was wrong with rsync cron jobs, the error:" + err.Error())
		}
	}

	uniqueDdTo := helpers.GetUniqueSlice(dTo)
	for _, v := range uniqueDdTo {
		_, err := s.Cron(os.Getenv("DELETE_CRON")).SingletonMode().Do(func(pathToDir string) {
			if err = helpers.DeleteFiles(pathToDir); err != nil {
				logger.Warn("Could not walk through " + pathToDir + ", the error: " + err.Error())
			}
		}, v)
		if err != nil {
			logger.Fatal("Something was wrong with deleting files in " + v + ", the error:" + err.Error())
		}
	}

	s.StartAsync()
}
