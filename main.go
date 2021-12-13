package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	mergeDir        = "" // 合并文件夹路径，可为.
	iPath           = "" // 待切割文件路径
	maxLen    int64 = 0  // 单文件最大长度
	offset    int64 = 0  // 文件偏移地址
	splitPart int64 = 2  // 切割份数
)

func RegExFindStringSubmatch(iStr, reStr string) []string { // 可公用
	ec, _ := regexp.Compile(reStr)
	return ec.FindStringSubmatch(iStr)
}

func doMergeDir() {
	// 查找*.n.1
	fis, _ := os.ReadDir(mergeDir)
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), ".1") { // xxx.ext.2.1
			ff := RegExFindStringSubmatch(fi.Name(), "(?smi)(.*?)\\.([0-9]+)\\.1")
			if "" == ff[2] {
				fmt.Println("# Error: 文件名不符合规则xxx.ext.2.1:", fi.Name())
				continue
			}
			fCount, err := strconv.Atoi(ff[2]) // 分割数
			if err != nil {
				fmt.Println("# Error: 文件名不符合规则xxx.ext.2.1:", fi.Name())
				continue
			}

			iModTime := time.Now()
			oName := ff[1]       // 合并后的文件名
			mFiles := []string{} // 待合并文件名列表
			var oFileLen int64 = 0
			for i := 1; i <= fCount; i++ {
				nowPath := filepath.Join(mergeDir, fmt.Sprintf("%s.%d.%d", oName, fCount, i))
				fi, err := os.Stat(nowPath)
				if err == nil {
					mFiles = append(mFiles, nowPath)
					oFileLen += fi.Size()
					iModTime = fi.ModTime()
				} else {
					fmt.Println("# Error: 文件不存在:", nowPath, err)
				}
			}
			if len(mFiles) != fCount {
				fmt.Printf("# Error: 分割文件: %s 不完整: 应有 %d 个文件，实际有 %d 个文件\n", oName, fCount, len(mFiles))
				continue
			}
			// 开始合并
			sTime := time.Now()
			oPath := filepath.Join(mergeDir, oName)
			oFile, err := os.Create(oPath)
			if err != nil {
				fmt.Println("# Error: CreateFile:", oPath, err)
				break
			}
			defer oFile.Close()
			var oWriteLen int64 = 0
			for i, iPath := range mFiles {
				fmt.Printf("- 写入 %d / %d: %s : ", 1+i, fCount, iPath)
				iFile, err := os.Open(iPath)
				if err != nil {
					fmt.Println("# Error: OpenFile:", iPath, err)
					return
				}
				defer iFile.Close()
				copyCount, _ := io.Copy(oFile, iFile)
				fmt.Println(copyCount)
				oWriteLen += copyCount
			}
			if oFileLen == oWriteLen {
				oFile.Close()
				os.Chtimes(oPath, time.Now(), iModTime)
				// eTime := int64(time.Since(sTime).Seconds()) // 速度太快（例如在内存盘里分割）耗时可能为0s
				fmt.Println("# 合并成功:", oPath, "大小:", oFileLen, "耗时:", time.Since(sTime).Seconds(), "\n")
			} else {
				fmt.Println("# 合并失败:", oPath, "应该大小:", oFileLen, "实际大小:", oWriteLen, "\n")
			}
		}
	}
}

// 功能: 将文件平均切割为小于4G的文件，便于上传到网盘
func main() {
	flag.StringVar(&iPath, "i", iPath, "待切割文件路径")
	flag.StringVar(&mergeDir, "m", mergeDir, "待合并文件夹路径，可为.")
	flag.Int64Var(&splitPart, "n", splitPart, "切割份数")
	flag.Parse()

	if "" != mergeDir { // 合并文件
		doMergeDir()
		return
	}

	if 1 == len(flag.Args()) {
		iPath = flag.Arg(0)
	}

	if "" == iPath {
		fmt.Println("用法:", os.Args[0], "待切割文件路径 | -m .")
		return
	}

	// 检查文件状态
	iFileInfo, err := os.Stat(iPath)
	if err != nil {
		if !os.IsExist(err) {
			fmt.Println("# Error: File Not Exist:", iPath)
		} else {
			fmt.Println("# Error: Stat File:", iPath, err)
		}
		return
	}

	// 打开文件
	iFile, err := os.Open(iPath)
	if err != nil {
		fmt.Println("# Error: OpenFile:", iPath, err)
		return
	}
	defer iFile.Close()

	sTime := time.Now()
	// 获取文件长度
	// iFileInfo, _ := iFile.Stat()
	iFileLen := iFileInfo.Size()
	iModTime := iFileInfo.ModTime()

	// 计算份数，得到单文件最大长度
	maxLen = int64(math.Ceil(float64(iFileLen) / float64(splitPart)))
	fmt.Println("# 单文件最大长度:", iFileLen, "/", splitPart, "=", maxLen)

	// 循环读取
	fileCount := 0
	for offset < iFileLen {
		fileCount += 1
		oPath := fmt.Sprintf("%s.%d.%d", iPath, splitPart, fileCount)

		oFile, err := os.Create(oPath)
		if err != nil {
			fmt.Println("# Error: CreateFile:", oPath, err)
			break
		}
		defer oFile.Close()

		fmt.Printf("- 写入 %d / %d: %s : ", fileCount, splitPart, oPath)
		copyCount, _ := io.CopyN(oFile, iFile, maxLen)
		fmt.Println(copyCount)
		// if err != nil {
		// 	fmt.Println("# Error: CopyBytes Len:", copyCount, err) // EOF
		// 	break
		// }

		offset += copyCount
		if copyCount < maxLen {
			if offset < iFileLen {
				fmt.Println("# Error: offset:", offset, "< inFileLen:", iFileLen)
				break
			}
		}

		oFile.Close()
		os.Chtimes(oPath, time.Now(), iModTime)
	}

	eTime := int64(time.Since(sTime).Seconds()) // 速度太快（例如在内存盘里分割）耗时可能为0s
	if 0 == eTime {
		fmt.Println("# 分割完毕, 耗时:", time.Since(sTime).Seconds(), "，速度真快")
	} else {
		fmt.Println("# 分割完毕, 耗时:", eTime, "速度:", iFileLen/1024/1024/eTime, "M/s")
	}
}
