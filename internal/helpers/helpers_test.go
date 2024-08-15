package helpers_test

import (
	"archive/zip"
	"fmt"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	"pullcsv/internal/helpers"
	"pullcsv/internal/logger"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bitfield/script"
	"github.com/google/go-cmp/cmp"
)

func provideFX() {
	fx.New(
		logger.WithZapLoggerFx(),
		fx.Invoke(func(logger *zap.Logger) {
		}),
	)
}

func TestIsOlderThan(t *testing.T) {
	t.Parallel()

	type testCase struct {
		want  bool
		t     time.Time
		hours int
	}

	now := time.Now()
	testCases := []testCase{
		{want: false, t: now.AddDate(0, 0, -2), hours: 49},
		{want: true, t: now.AddDate(0, 0, -1), hours: 5},
	}

	for _, tc := range testCases {
		got := helpers.IsOlderThan(tc.t, tc.hours)
		if tc.want != got {
			t.Errorf("Expected: %v, got: %v", tc.want, got)
		}
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestUnarchiveFileSrcDstExist(t *testing.T) {
	t.Parallel()

	type testCase struct {
		src, dst string
	}

	testCases := []testCase{
		{src: "../../forTests/README.md.zip", dst: "/tmp/"},
	}

	for _, tc := range testCases {
		err := helpers.UnarchiveFile(tc.src, tc.dst)
		if err != nil {
			t.Errorf("SRC = %s, DTS = %s. ERROR: %v", tc.src, tc.dst, err.Error())
		}
	}
}

func TestUngzipFile(t *testing.T) {
	t.Parallel()

	type testCase struct {
		source, target string
	}

	testCases := []testCase{
		{source: "../../forTests/README.md.gz", target: "/tmp/"},
	}

	for _, tc := range testCases {
		err := helpers.UngzipFile(tc.source, tc.target)
		if err != nil {
			t.Errorf("SRC = %s, DTS = %s. ERROR: %v", tc.source, tc.target, err.Error())
		}
	}
}

func TestEnvReplacement(t *testing.T) {
	t.Parallel()

	now := time.Now()
	yesterdayNofmt := now.AddDate(0, 0, -1)
	todayWith := now.Format("2006-01-02")
	todayWithout := now.Format("20060102")
	yesterdayWith := yesterdayNofmt.Format("2006-01-02")
	yesterdayWithout := yesterdayNofmt.Format("20060102")

	type testCase struct {
		dFromSource, dFromResult string
	}

	testCases := []testCase{
		{
			dFromSource: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*_TODAY_*",
			dFromResult: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*" + todayWithout + "*",
		},
		{
			dFromSource: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*_YESTERDAY_*",
			dFromResult: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*" + yesterdayWithout + "*",
		},
		{
			dFromSource: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*_TO-DAY_*",
			dFromResult: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*" + todayWith + "*",
		},
		{
			dFromSource: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*_YES-TER-DAY_*",
			dFromResult: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*" + yesterdayWith + "*",
		},
	}

	for _, tc := range testCases {
		got := helpers.EnvReplacement(tc.dFromSource)
		if tc.dFromResult != got {
			t.Errorf("Expected: %s, got: %s", tc.dFromResult, got)
		}
	}
}

func TestGetRsyncExitCodeMeaning(t *testing.T) {
	t.Parallel()

	type testCase struct {
		code    int
		meaning string
	}

	testCases := []testCase{
		{code: 0, meaning: "Success"},
		{code: 1, meaning: "Syntax or usage error"},
		{code: 2, meaning: "Protocol incompatibility"},
		{code: 3, meaning: "Errors selecting input/output files, dirs"},
		{code: 4, meaning: "Requested action not supported: an attempt was made to manipulate 64-bit files on a platform that cannot support them; or an option was specified that is supported by the client and not by the server."},
		{code: 5, meaning: "Error starting client-server protocol"},
		{code: 6, meaning: "Daemon unable to append to log-file"},
		{code: 10, meaning: "Error in socket I/O (maybe there is a problem with DNS resolution)"},
		{code: 11, meaning: "Error in file I/O (maybe there is no destination)"},
		{code: 12, meaning: "Error in rsync protocol data stream"},
		{code: 13, meaning: "Errors with program diagnostics"},
		{code: 14, meaning: "Error in IPC code"},
		{code: 20, meaning: "Received SIGUSR1 or SIGINT"},
		{code: 21, meaning: "Some error returned by waitpid()"},
		{code: 22, meaning: "Error allocating core memory buffers"},
		{code: 23, meaning: "Partial transfer due to error (maybe there are not files on remote side by the mask)"},
		{code: 24, meaning: "Partial transfer due to vanished source files"},
		{code: 25, meaning: "The --max-delete limit stopped deletions"},
		{code: 30, meaning: "Timeout in data send/receive"},
		{code: 35, meaning: "Timeout waiting for daemon connection"},
		{code: 100500, meaning: "Unknown"},
		{code: 27, meaning: "Unknown"},
	}

	for _, tc := range testCases {
		got := helpers.GetRsyncExitCodeMeaning(tc.code)
		if tc.meaning != got {
			t.Errorf("Want: %s, got: %s", tc.meaning, got)
		}
	}
}

func TestSaveExcludeFileInvalid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		want            []string
		fileSlice, exFN string
	}

	testCases := []testCase{
		{fileSlice: "../../forTests/SaveExcludeFile/case0/", exFN: "../doesntexist"},
		{fileSlice: "doesntexist", exFN: "../doesntexist"},
		{fileSlice: "doesntexist", exFN: "../../forTests/SaveExcludeFile/exFNcase1"},
	}

	var i int
	for _, tc := range testCases {
		_, err := helpers.SaveExcludeFile(tc.fileSlice, tc.exFN)
		if err == nil {
			t.Error("want error for invalid input, got nil")
		}
		i++
	}
}

func TestSaveExcludeFile(t *testing.T) {
	t.Parallel()

	os.Mkdir("../../forTests/SaveExcludeFile/case1/", 0755)
	os.Mkdir("../../forTests/SaveExcludeFile/case2/", 0755)

	type testCase struct {
		want            []string
		fileSlice, exFN string
	}

	wantCase0 := []string{"f1", "f2", "f3"}
	wantCase1 := []string{"first", "second", "third", "something"}
	wantCase2 := []string{}
	wantCase3 := []string{"f1", "f2", "f3", "first", "second", "third", "something"}
	wantCase4 := []string{"case4-f1"}

	sort.Strings(wantCase0)
	sort.Strings(wantCase1)
	sort.Strings(wantCase3)

	testCases := []testCase{
		{fileSlice: "../../forTests/SaveExcludeFile/case0/", exFN: "../../forTests/SaveExcludeFile/exFNcase0", want: wantCase0},
		{fileSlice: "../../forTests/SaveExcludeFile/case1/", exFN: "../../forTests/SaveExcludeFile/exFNcase1", want: wantCase1},
		{fileSlice: "../../forTests/SaveExcludeFile/case2/", exFN: "../../forTests/SaveExcludeFile/exFNcase2", want: wantCase2},
		{fileSlice: "../../forTests/SaveExcludeFile/case3/", exFN: "../../forTests/SaveExcludeFile/exFNcase3", want: wantCase3},
		{fileSlice: "../../forTests/SaveExcludeFile/case4/", exFN: "../../forTests/SaveExcludeFile/exFNcase4", want: wantCase4},
	}

	var i int
	for _, tc := range testCases {
		got, _ := helpers.SaveExcludeFile(tc.fileSlice, tc.exFN)
		if !stringSlicesEqual(tc.want, got) {
			t.Errorf("Case %d, Want: %s, got: %s", i, tc.want, got)
		}
		i++
	}

	os.Remove("../../forTests/SaveExcludeFile/case1")
	os.Remove("../../forTests/SaveExcludeFile/case2")

}

func TestGetExludeFileNameWrongENV(t *testing.T) {
	os.Setenv("STAND_NAME", "dev25")

	type testCase struct {
		podName, dFrom, dTo string
	}

	testCases := []testCase{
		{podName: "23f34f3FR4Fwrf23rf23w", dFrom: "rsync://USERNAME@server-name/pullcsv/some-files/*csv", dTo: "doesntmatterhere"},
		{podName: "some-pod-name-5448486d5c11-qjpvq", dFrom: "rsync://USERNAME@server-name/pullcsv/some-files/*csv", dTo: "doesntmatterhere"},
		{podName: "some-pod-name-12345678-qjpvq", dFrom: "rsync23r23r23r", dTo: "doesntmatterhere"},
		{podName: "some-pod-name-123456789-qjpvq", dFrom: "rsync://v3534g435g3wllcsv/some-files/*csv", dTo: "doesntmatterhere"},
		{podName: "some-pod-name-5448486d5c-qjpvq", dFrom: "rsync____", dTo: "doesntmatterhere"},
		{podName: "some-pod-name-5448486d5c-qjpvq", dFrom: "rsync-----23r23", dTo: "doesntmatterhere"},
	}

	for _, tc := range testCases {
		os.Setenv("POD_NAME", tc.podName)
		_, err := helpers.GetExludeFileName(tc.dFrom, tc.dTo)
		if err == nil {
			t.Error("Want error (because of wrong POD_NAME), got nil")
		}
	}
}

func TestGetExludeFileName(t *testing.T) {
	os.Setenv("STAND_NAME", "dev25")

	type testCase struct {
		want, dFromPath, dToPath string
	}

	testCases := []testCase{

		{dFromPath: "rsync://USERNAME@server-name/pullcsv/some-files/*csv", dToPath: "/path_in_pod/csv/in/", want: "some-pod-name_prices_csv-dev25_backend-products_csv_in_-excludeFile"},
		{dFromPath: "rsync://USERNAME@server-name/pullcsv/some-files2/*_TODAY_*csv", dToPath: "/path_in_pod/csv/in/", want: "some-pod-name_pim__TODAY_csv-dev25_backend-products_csv_in_-excludeFile"},
		{dFromPath: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/FULLSTOCK*_TODAY_*", dToPath: "/backend-products/stocks/stocks-shops/in/", want: "some-pod-name_shops_stocks_rf_FULLSTOCK_TODAY_-dev25_backend-products_stocks_stocks-shops_in_-excludeFile"},
		{dFromPath: "rsync://USERNAME@server-name/pullcsv/shops_stocks_rf/DELTASTOCK*_TODAY_*", dToPath: "/backend-products/stocks/stocks-shops/in/", want: "some-pod-name_shops_stocks_rf_DELTASTOCK_TODAY_-dev25_backend-products_stocks_stocks-shops_in_-excludeFile"},
		{dFromPath: "rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_TODAY_*", dToPath: "/path_in_pod/stocks/in/", want: "some-pod-name_some_mask_some_part_of_the_file_TODAY_-dev25_backend-products_stocks_stocks-im_in_-excludeFile"},
	}

	var i int
	for _, tc := range testCases {
		os.Setenv("POD_NAME", "some-pod-name-5448486d5c-qjpvq")
		got, _ := helpers.GetExludeFileName(tc.dFromPath, tc.dToPath)
		print(got + "\n")
		if tc.want != got {
			t.Errorf("Case %d, Want: %s, got: %s", i, tc.want, got)
		}
		i++
	}
}

func TestGetRandStr(t *testing.T) {
	t.Parallel()

	type testCase struct {
		want int
	}

	testCases := []testCase{
		{want: 10},
		{want: 19},
		{want: 121},
		{want: 1},
	}

	for _, tc := range testCases {
		got := helpers.GetRandStr(tc.want)
		if tc.want != len(got) {
			t.Errorf("Want: %d, got: %s", tc.want, got)
		}
	}
}

func TestLogEveryFileAndMoveIt(t *testing.T) {
	t.Parallel()
	provideFX()

	os.Mkdir("/tmp/LogEveryFileAndMoveIt", 0755)
	os.Mkdir("/tmp/LogEveryFileAndMoveIt/random123", 0755)
	os.Create("/tmp/LogEveryFileAndMoveIt/random123/file1")
	os.Create("/tmp/LogEveryFileAndMoveIt/random123/file2")
	os.Create("/tmp/LogEveryFileAndMoveIt/random123/file3")

	helpers.LogEveryFileAndMoveIt("/tmp/LogEveryFileAndMoveIt/", "/tmp/LogEveryFileAndMoveIt/random123/")

	sl := []string{
		"/tmp/LogEveryFileAndMoveIt/file1",
		"/tmp/LogEveryFileAndMoveIt/file2",
		"/tmp/LogEveryFileAndMoveIt/file3",
		"/tmp/LogEveryFileAndMoveIt/random123",
	}
	filesIndTo, _ := script.ListFiles("/tmp/LogEveryFileAndMoveIt/").Slice()
	sort.Strings(filesIndTo)

	if !cmp.Equal(sl, filesIndTo) {
		t.Error("sl and filesIndTo aren'r equal")
	}

	os.RemoveAll("/tmp/LogEveryFileAndMoveIt")
}

func TestDeleteFilesOlderThan(t *testing.T) {
	t.Parallel()
	provideFX()

	os.Mkdir("/tmp/TestDeleteFilesOlderThan", 0755)
	os.Create("/tmp/TestDeleteFilesOlderThan/file1")
	os.Create("/tmp/TestDeleteFilesOlderThan/file2")
	os.Create("/tmp/TestDeleteFilesOlderThan/file3")

	now := time.Now()
	mtime := now.Add(-49 * time.Hour)
	atime := now.Add(-49 * time.Hour)
	os.Chtimes("/tmp/TestDeleteFilesOlderThan/file1", atime, mtime)
	os.Chtimes("/tmp/TestDeleteFilesOlderThan/file2", atime, mtime)
	os.Chtimes("/tmp/TestDeleteFilesOlderThan/file3", atime, mtime)

	emptySl := []string{
		"",
	}
	helpers.DeleteFiles("/tmp/TestDeleteFilesOlderThan/")
	filesIndDir, _ := script.ListFiles("/tmp/TestDeleteFilesOlderThan/").Slice()
	if !cmp.Equal(emptySl, filesIndDir) {
		t.Error("sl and filesIndDir aren'r equal")
	}
	os.RemoveAll("/tmp/TestDeleteFilesOlderThan")
}

func TestDeleteFilesPartialRsync(t *testing.T) {
	t.Parallel()
	provideFX()

	os.Mkdir("/tmp/TestDeleteFilesPartialRsync", 0755)
	os.Create("/tmp/TestDeleteFilesPartialRsync/.file1.aw3dfq")
	os.Create("/tmp/TestDeleteFilesPartialRsync/.file2.3cav02")
	os.Create("/tmp/TestDeleteFilesPartialRsync/.file3.2wdc49")

	now := time.Now()
	mtime := now.Add(-5 * time.Hour)
	atime := now.Add(-5 * time.Hour)
	os.Chtimes("/tmp/TestDeleteFilesPartialRsync/.file1.aw3dfq", atime, mtime)
	os.Chtimes("/tmp/TestDeleteFilesPartialRsync/.file2.3cav02", atime, mtime)
	os.Chtimes("/tmp/TestDeleteFilesPartialRsync/.file3.2wdc49", atime, mtime)

	emptySl := []string{
		"",
	}
	helpers.DeleteFiles("/tmp/TestDeleteFilesPartialRsync/")
	filesIndDir, _ := script.ListFiles("/tmp/TestDeleteFilesPartialRsync/").Slice()
	if !cmp.Equal(emptySl, filesIndDir) {
		t.Error("emptySl and filesIndDir aren'r equal")
	}
	os.RemoveAll("/tmp/TestDeleteFilesPartialRsync")
}

func TestRsync(t *testing.T) {
	t.Parallel()
	provideFX()

	type testCase struct {
		want     int
		from, to string
	}

	testCases := []testCase{
		{want: 0, from: "/tmp/TestRsync/From/file1", to: "/tmp/TestRsync/To/"},
		{want: 23, from: "/blablabla", to: "/proc/"},
	}

	os.Mkdir("/tmp/TestRsync", 0755)
	os.Mkdir("/tmp/TestRsync/From", 0755)
	os.Mkdir("/tmp/TestRsync/To", 0755)
	os.Create("/tmp/TestRsync/From/file1")

	for _, tc := range testCases {
		if got := helpers.Rsync("/usr/bin/rsync " + tc.from + " " + tc.to); got != tc.want {
			t.Errorf("Expected: %v, got: %v", tc.want, got)
		}
	}

	os.RemoveAll("/tmp/TestRsync")
}

func TestWorkWithArchivesInvalid(t *testing.T) {
	t.Parallel()
	provideFX()

	err := helpers.WorkWithArchives("../../forTests/WorkWithArchivesInvalid/")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestTruncateExcludeFileInvalid(t *testing.T) {
	t.Parallel()

	err := helpers.TruncateExcludeFile("/tmp/netutakogoslovanetu", 1, 2, 20)
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestTruncateExcludeFile(t *testing.T) {
	t.Parallel()

	src := "../../forTests/SaveExcludeFile/exFNcase3"
	dest := "../../forTests/TestTruncateExcludeFile"

	bytesRead, err := ioutil.ReadFile(src)

	if err != nil {
		t.Fatal(err)
	}

	err = ioutil.WriteFile(dest, bytesRead, 0644)

	if err != nil {
		t.Fatal(err)
	}

	sl := []string{
		"third",
		"something",
	}

	err = helpers.TruncateExcludeFile("../../forTests/TestTruncateExcludeFile", 1, 2, 20)
	if err != nil {
		t.Errorf("want nil, got error: %v\n", err)
	}

	linesInFile, _ := script.File("../../forTests/TestTruncateExcludeFile").Slice()

	if !cmp.Equal(sl, linesInFile) {
		t.Errorf("sl and linesInFile aren't equal")
	}

	os.Remove("../../forTests/TestTruncateExcludeFile")
}

func TestAddSeparator(t *testing.T) {
	t.Parallel()

	s4test := "/lalala /wwefv/wefwef /23rrf/wef54h/th4th"

	want := []string{
		"/lalala/",
		"/wwefv/wefwef/",
		"/23rrf/wef54h/th4th/",
	}

	got, _ := helpers.AddSeparator(s4test)
	if !cmp.Equal(got, want) {
		t.Errorf("want: %v,\n got: %v", want, got)
	}
}

func TestPrepareEnvInvalid(t *testing.T) {
	t.Parallel()
	os.Unsetenv("RSYNC_PASSWORD")
	_, _, _, _, err := helpers.PrepareEnv()
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestGetFileSizeInvalid(t *testing.T) {
	t.Parallel()

	_, err := helpers.GetFileSize("ololo123")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestPrepareEnv(t *testing.T) {
	t.Parallel()
	os.Setenv("DOWNLOAD_FROM", "rsync://USERNAME@server-name/pullcsv/some-files/*_TODAY_*csv rsync://USERNAME@server-name/pullcsv/some-files2/*_TODAY_*csv")
	os.Setenv("DOWNLOAD_TO", "/path_in_pod/csv/in/ /path_in_pod/csv/in/")
	os.Setenv("POD_NAME", "some-pod-name-5448486d5c-qjpvq")
	os.Setenv("STAND_NAME", "dev25")
	os.Setenv("RSYNC_PASSWORD", "123")

	type testCase struct {
		wantDfrom, wantDto         []string
		wantstandName, wantpodName string
		err                        error
	}

	testCases := []testCase{
		{
			wantDfrom: []string{
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TODAY_*csv",
				"rsync://USERNAME@server-name/pullcsv/some-files2/*_TODAY_*csv",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TO-DAY_*csv",
				"rsync://USERNAME@server-name/pullcsv/some-files2/*_TO-DAY_*csv",
			},
			wantDto: []string{
				"/path_in_pod/csv/in/",
				"/path_in_pod/csv/in/",
				"/path_in_pod/csv/in/",
				"/path_in_pod/csv/in/",
			},
			wantstandName: "dev25",
			wantpodName:   "some-pod-name-5448486d5c-qjpvq",
			err:           nil,
		},
	}

	for _, tc := range testCases {
		dFrom, dTo, standName, podName, err := helpers.PrepareEnv()

		if err != nil {
			t.Errorf("Ckeck err, want: %v, got: %v", tc.err, err)
		}

		if !cmp.Equal(tc.wantDfrom, dFrom) {
			t.Errorf("Ckeck dFrom, want: %v, got: %v", tc.wantDfrom, dFrom)
		}

		if !cmp.Equal(tc.wantDto, dTo) {
			t.Errorf("Ckeck dTo, want: %v, got: %v", tc.wantDto, dTo)
		}

		if tc.wantstandName != standName {
			t.Errorf("Ckeck standName, want: %v, got: %v", tc.wantstandName, standName)
		}

		if tc.wantpodName != podName {
			t.Errorf("Ckeck podName, want: %v, got: %v", tc.wantpodName, podName)
		}
	}
}

func TestGetUniqueSlice(t *testing.T) {
	t.Parallel()

	want := []string{
		"123",
		"321",
		"111",
	}

	sl4test := []string{
		"123",
		"321",
		"111",
		"123",
		"321",
		"111",
		"123",
		"321",
		"111",
	}

	got := helpers.GetUniqueSlice(sl4test)
	if !cmp.Equal(want, got) {
		t.Errorf("want: %v, got: %v", want, got)
	}
}

func TestGetFileSize(t *testing.T) {
	t.Parallel()

	type testCase struct {
		want     int64
		filePath string
	}

	testCases := []testCase{
		{want: 48410, filePath: "../../forTests/fileSizeCountLines"},
	}

	for _, tc := range testCases {
		got, _ := helpers.GetFileSize(tc.filePath)
		if tc.want != got {
			t.Errorf("Want: %d, got: %d", tc.want, got)

		}
	}
}

func TestGetCountLines(t *testing.T) {
	t.Parallel()

	type testCase struct {
		want     int
		filePath string
	}

	testCases := []testCase{
		{want: 1000, filePath: "../../forTests/fileSizeCountLines"},
	}

	for _, tc := range testCases {
		got, _ := helpers.GetCountLines(tc.filePath)
		if tc.want != got {
			t.Errorf("Want: %d, got: %d", tc.want, got)

		}
	}
}

func TestExists(t *testing.T) {
	t.Parallel()

	if e := helpers.Exists("non-existent"); e {
		t.Error("non-existent file should not exist")
	}
	tmpFile := filepath.Join("/tmp/", "gct-test.txt")
	if err := ioutil.WriteFile(tmpFile, []byte("hello world"), os.ModeAppend); err != nil {
		t.Fatal(err)
	}
	if e := helpers.Exists(tmpFile); !e {
		t.Error("file should exist")
	}
	if err := os.Remove(tmpFile); err != nil {
		t.Errorf("unable to remove %s, manual deletion is required", tmpFile)
	}
}

func TestMove(t *testing.T) {
	t.Parallel()

	tester := func(in, out string, write bool) error {
		if write {
			if err := ioutil.WriteFile(in, []byte("PullCSV"), 0770); err != nil {
				return err
			}
		}

		if err := helpers.Move(in, out); err != nil {
			return err
		}

		contents, err := ioutil.ReadFile(out)
		if err != nil {
			return err
		}

		if !strings.Contains(string(contents), "PullCSV") {
			return fmt.Errorf("unable to find previously written data")
		}

		return os.Remove(out)
	}

	type testTable struct {
		InFile      string
		OutFile     string
		Write       bool
		ErrExpected bool
	}

	var tests []testTable
	switch runtime.GOOS {
	case "windows":
		tests = []testTable{
			{InFile: "*", OutFile: "gct.txt", Write: true, ErrExpected: true},
			{InFile: "*", OutFile: "gct.txt", Write: false, ErrExpected: true},
			{InFile: "in.txt", OutFile: "*", Write: true, ErrExpected: true},
		}
	default:
		tests = []testTable{
			{InFile: "", OutFile: "gct.txt", Write: true, ErrExpected: true},
			{InFile: "", OutFile: "gct.txt", Write: false, ErrExpected: true},
			{InFile: "in.txt", OutFile: "", Write: true, ErrExpected: true},
		}
	}
	tests = append(tests, []testTable{
		{InFile: "in.txt", OutFile: "gct.txt", Write: true, ErrExpected: false},
		{InFile: "in.txt", OutFile: "non-existing/gct.txt", Write: true, ErrExpected: false},
		{InFile: "in.txt", OutFile: "in.txt", Write: true, ErrExpected: false},
	}...)

	if helpers.Exists("non-existing") {
		t.Error("target 'non-existing' should not exist")
	}
	defer os.RemoveAll("non-existing")
	defer os.Remove("in.txt")

	for x := range tests {
		err := tester(tests[x].InFile, tests[x].OutFile, tests[x].Write)
		if err != nil && !tests[x].ErrExpected {
			t.Errorf("Test %d failed, unexpected err %s\n", x, err)
		}
	}
}

func TestMoveInvalid(t *testing.T) {
	t.Parallel()

	err := helpers.Move("/ewdewd", "/d34f34fv")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestUnzipSourceInvalid(t *testing.T) {
	t.Parallel()

	err := helpers.UnzipSource("ewdewd", "d34f34fv")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestUngzipFileInvalid(t *testing.T) {
	t.Parallel()

	err := helpers.UngzipFile("ololo", "d34f34fv")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestUnarchiveFileInvalid(t *testing.T) {
	t.Parallel()

	err := helpers.UnarchiveFile("ololo", "d34f34fv")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestGetCountLinesInvalid(t *testing.T) {
	t.Parallel()

	_, err := helpers.GetCountLines("d34f34fv")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestUnzipFileInvalid(t *testing.T) {
	t.Parallel()
	f := new(zip.File)
	err := helpers.UnzipFile(f, "/dev/null/d34f34fv")
	if err == nil {
		t.Error("want error for invalid input, got nil")
	}
}

func TestDuplicateEnvs(t *testing.T) {
	t.Parallel()

	type testCase struct {
		dFrom,
		dTo,
		wantDfrom,
		wantDto,
		wantConcat []string
	}

	testCases := []testCase{
		{
			dFrom: []string{
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TODAY_*",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_YESTERDAY_*",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_TODAY_*",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_YESTERDAY_*",
			},
			dTo: []string{
				"/path_in_pod/csv/in/",
				"/path_in_pod/csv/in/",
				"/path_in_pod/stocks/in/",
				"/path_in_pod/stocks/in/",
			},
			wantDfrom: []string{
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TODAY_*",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_YESTERDAY_*",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_TODAY_*",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_YESTERDAY_*",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TO-DAY_*",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_YES-TER-DAY_*",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_TO-DAY_*",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_YES-TER-DAY_*",
			},
			wantDto: []string{
				"/path_in_pod/csv/in/",
				"/path_in_pod/csv/in/",
				"/path_in_pod/stocks/in/",
				"/path_in_pod/stocks/in/",
				"/path_in_pod/csv/in/",
				"/path_in_pod/csv/in/",
				"/path_in_pod/stocks/in/",
				"/path_in_pod/stocks/in/",
			},
			wantConcat: []string{
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TODAY_*" + "/path_in_pod/csv/in/",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_YESTERDAY_*" + "/path_in_pod/csv/in/",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_TODAY_*" + "/path_in_pod/stocks/in/",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_YESTERDAY_*" + "/path_in_pod/stocks/in/",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_TO-DAY_*" + "/path_in_pod/csv/in/",
				"rsync://USERNAME@server-name/pullcsv/some-files/*_YES-TER-DAY_*" + "/path_in_pod/csv/in/",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_TO-DAY_*" + "/path_in_pod/stocks/in/",
				"rsync://USERNAME@server-name/pullcsv/some_path/some_part_of_the_file*_YES-TER-DAY_*" + "/path_in_pod/stocks/in/",
			},
		},
	}

	for _, tc := range testCases {
		helpers.DuplicateEnvs(&tc.dFrom, &tc.dTo)
		if len(tc.wantDfrom) != len(tc.dFrom) {
			t.Errorf("Lenght of wantDfrom must be equal Lenght of dFrom")
		}

		if len(tc.wantDto) != len(tc.dTo) {
			t.Errorf("Lenght of wantDto must be equal Lenght of dTo")
		}

		for i := 0; i < len(tc.dFrom); i++ {
			if tc.dFrom[i] != tc.wantDfrom[i] {
				t.Errorf("dFrom: %s, is not equal wantDfrom: %s", tc.dFrom[i], tc.wantDfrom[i])
			}
			if tc.dTo[i] != tc.wantDto[i] {
				t.Errorf("dTo: %s, is not equal wantDto: %s", tc.dTo[i], tc.wantDto[i])
			}
			if tc.dFrom[i]+tc.dTo[i] != tc.wantConcat[i] {
				t.Errorf("dFrom+dTo: %s%s, is not equal wantConcatDfromDto: %s", tc.dFrom[i], tc.dTo[i], tc.wantConcat[i])
			}
		}
	}
}

func TestGetOldestNewestCountFiles(t *testing.T) {
	t.Parallel()
	provideFX()

	type testCase struct {
		newest, oldest int64
		count          int
	}

	testCases := []testCase{
		{newest: 1614291016, oldest: 1612044592, count: 10},
	}

	for _, tc := range testCases {
		gotNewest, gotOldest, gotCount := helpers.GetOldestNewestCountFiles("/usr/share/pam/")
		if tc.newest != gotNewest {
			t.Errorf("Want newest: %d, got: %d", tc.newest, gotNewest)
		} else if tc.oldest != gotOldest {
			t.Errorf("Want oldest: %d, got: %d", tc.oldest, gotOldest)
		} else if tc.count != gotCount {
			t.Errorf("Want count: %d, got: %d", tc.count, gotCount)
		}
	}
}
