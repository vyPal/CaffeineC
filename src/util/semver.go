package util

import (
	"fmt"
	"strconv"
	"strings"
)

type Semver struct {
	Major      int
	Minor      int
	Patch      int
	Beta       bool
	Alpha      bool
	Prerelease int
}

func Parse(semver string) (Semver, error) {
	s := Semver{}
	split := strings.Split(semver, ".")
	major, err := strconv.Atoi(split[0])
	if err != nil {
		return Semver{}, err
	}
	s.Major = major

	minor, err := strconv.Atoi(split[1])
	if err != nil {
		return Semver{}, err
	}
	s.Minor = minor

	patch := strings.Split(split[2], "-")
	patchNum, err := strconv.Atoi(patch[0])
	if err != nil {
		return Semver{}, err
	}
	s.Patch = patchNum

	if len(patch) > 1 {
		if strings.Contains(patch[1], "beta") {
			s.Beta = true
			prerelease := strings.Split(patch[1], ".")
			prereleaseNum, err := strconv.Atoi(prerelease[1])
			if err != nil {
				return Semver{}, err
			}
			s.Prerelease = prereleaseNum
		} else if strings.Contains(patch[1], "alpha") {
			s.Alpha = true
			prerelease := strings.Split(patch[1], ".")
			prereleaseNum, err := strconv.Atoi(prerelease[1])
			if err != nil {
				return Semver{}, err
			}
			s.Prerelease = prereleaseNum
		} else {
			return Semver{}, fmt.Errorf("invalid prerelease type: %s", patch[1])
		}
	}

	return s, nil
}

func (s Semver) String() string {
	str := strconv.Itoa(s.Major) + "." + strconv.Itoa(s.Minor) + "." + strconv.Itoa(s.Patch)
	if s.Beta {
		str += "-beta." + strconv.Itoa(s.Prerelease)
	} else if s.Alpha {
		str += "-alpha." + strconv.Itoa(s.Prerelease)
	}
	return str
}

func (s Semver) Satisfies(cmp string) (bool, error) {
	tilde := strings.HasPrefix(cmp, "~")
	caret := strings.HasPrefix(cmp, "^")
	gt := strings.HasPrefix(cmp, ">")
	lt := strings.HasPrefix(cmp, "<")
	if tilde || caret || gt || lt {
		cmp = cmp[1:]
	}

	c, err := Parse(cmp)
	if err != nil {
		return false, err
	}

	if (tilde && (c.Major != s.Major || c.Minor != s.Minor || c.Patch > s.Patch)) ||
		(caret && (c.Major != s.Major || c.Minor > s.Minor || c.Patch > s.Patch)) ||
		(gt && (c.Major > s.Major || c.Minor > s.Minor || c.Patch > s.Patch)) ||
		(lt && (c.Major < s.Major || c.Minor < s.Minor || c.Patch < s.Patch)) ||
		(!tilde && !caret && !gt && !lt && (c.Major != s.Major || c.Minor != s.Minor || c.Patch != s.Patch)) {
		return false, nil
	}

	return true, nil
}
