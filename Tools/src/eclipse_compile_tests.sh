# hack: to have tests compile in eclipse, we use _testt.go instead of _test.go as suffix file names
# NOTE: this script has to be executed from the project root
find src/aws-ssm-agent -name "*_test.go*" | sed 's/\(.*\)_test.go/mv \1_test.go \1_testt.go/' | while read line ; do echo $line; `$line`; done #| sh

