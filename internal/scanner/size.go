package scanner

import "os"

// logicalSize 함수 하나만 담은 작은 유틸 파일이다. scanner.go의
// entryFromInfo가 디렉터리도 심볼릭 링크/reparse point도 아닌 일반 파일의
// 크기를 계산할 때 사용하며, 일부 플랫폼에서 발생할 수 있는 비정상적인
// 음수 크기 값을 방어적으로 0으로 처리한다. 이런 소소하지만 여러 곳에서
// 재사용될 수 있는 크기 계산 로직을 scanner.go 본체와 분리해 둔 파일이다.
func logicalSize(info os.FileInfo) int64 {
	if info.IsDir() || info.Size() < 0 {
		return 0
	}
	return info.Size()
}
