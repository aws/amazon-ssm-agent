#!/usr/bin/env bash
	
# Static security analysis script
#
# Installation & Use:
# 1. install git for file change tracking
# 2. Install scans. gosec and govulncheck scans must be installed to pass approval workflow. golangci-lint and staticcheck are recommended but optional linters
# |- gosec
# |- govulncheck
# |- staticcheck
# |- golangci-lint
# 3. It is very important that the version of go installed matches the one used by the makefile! Some of these scanner use the go version to check for errors
# 4. Configuration for staticcheck and golangci-lint can be created by adding in staticcheck.conf and .golangci.yml files
# 5. Use the -h options for help with this tool
# 6. Run make analyze to use recommended options. (Approvol workflow uses this target!)
# 7. Using -o=dir or -o=file will create formated markdown files

# Configurations Information
#======================================================================================================

# Packages that will be scanned when the -d (--default) flag is set. 
# This list of directories where choosen as packages that are maintained by SSM agent team
DEFAULT_PACKAGES="./agent ./core ./common ./internal"

# Names of each scanning library that is avalible to use. Code must pass govulncheck and gosec scans
# Note that the index must match for NAMES, FLAGS, TEST_FLAGS, SCANNER_INSTALLATION_URL
NAMES=(
  "govulncheck"
  "gosec"
  # "staticcheck"    
  # "golangci-lint"
)

# Default flags used to run command. Index correspond to the scan above
FLAGS=(
  "" 
  "-quiet -severity high -confidence high" 
  # "" 
  # "run"
)

# Additional flags that are enabled when -t (--test) flag is set
TEST_FLAGS=(
  "-test"
  "-tests"
  # "-test"
  # ""
) 

# Installation URLs. Go must be > 1.16 to use go install command
# TODO: Update gosec install command to "go install github.com/securego/gosec/v2/cmd/gosec@latest"
SCANNER_INSTALLATION_URL=(
  "go install golang.org/x/vuln/cmd/govulncheck@latest"
  "curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $(go env GOPATH)/bin"
  # "go install honnef.co/go/tools/cmd/staticcheck@v0.3.1"
  # "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
)
#=======================================================================================================

# Update path to include installed binaries
PATH=${PATH}:$(go env GOPATH)/bin

# Color and text constants for shell. Checks that we are not inside a dumb terminal which has no color features
if [[ ${TERM} != "dumb" ]]; then
  RED=$(tput setaf 1) 
  GREEN=$(tput setaf 2) 
  YELLOW=$(tput setaf 3) 
  BLUE=$(tput setaf 4)
  MAGENTA=$(tput setaf 5)
  CYAN=$(tput setaf 6)
  RESET_COLOR=$(tput init) 
  BOLD=$(tput bold)
  UNDERLINE=$(tput smul)
  NORMAL=$(tput sgr0)
fi

# Help Message
help="
${YELLOW}This script uses multiple static security analysis libraries to scan and find known CVEs and vulnerabilities within golang packages${RESET_COLOR}\n
   ${BOLD}${UNDERLINE}Usage${NORMAL}:
\t$(basename $0) [options] [files...]
   ${BOLD}${UNDERLINE}Options${NORMAL}:
\t-s  --scanner=\"arg...\"    List of security scanners that will be used currently there ${SCANNERS} are avalible
\t    --[scanner]=\"flags\"   Pass in additional command flags to scanner
\t-d  --default             Runs scans on specified default locations
\t-f  --fail                Prevents script from exiting after first failure
\t-r  --rel=<path>          Set the path all commands are run relative to
\t-i  --install             Installs latest scanners automatically if missing
\t-I                        Installs scanners depenedencies and exits          
\t-q  --quiet               Disables additional prints
\t-c  --color               Disables color output 
\t-t  --tests               Enable scanning on test code
\t-o  --out=<file|dir>      Set location for debugging output. Defaults to console
\t-n  --name=\"name\"         Names of the output file/directory
\t${GREEN}-h  --help                Help information${RESET_COLOR}
   ${BOLD}${UNDERLINE}Arguments${NORMAL}:
\t[files...]                Defaults to ./... which recursively scans all subpackages of the project 
"

# Toggleable echo function. If -q (--quiet) is set we do not echo logging messages
print() {
  [[ ${option_q} == true ]] || echo -e $@
}

# Returns the index of scanner from NAMES. if output is > array length, element is not contained inside the list
# $1: element we are searching for inside of $NAMES[@]
# Returns: index within $NAMES if $1 is found otherwise len($NAMES)
indexOf() {    
  for i in ${!NAMES[@]}; do
    if [ ${NAMES[$i]} == $1 ]; then
        return $i
    fi
  done
  return ${#NAMES[@]}
}

# Default option values
option_s=${NAMES[@]}  # List of scanners that will be used this run (defaults to all of them)
option_d=false        # Use default scan locations
option_f=true         # Prevents program from exiting after a scanner finds error
option_i=false        # Installs recommended versions of scann
option_q=false        # Runs script in quiet mode
option_t=false        # Also scan test files
option_o=""           # Sets the debug log output location. Default to the terminal

name=$(basename $0)
filename=${name%???}_results

# Args parsing for arguments in the form -x="a b c ..."
# $1 is the prefix: (x)
# $2 is the suffix: "a b c ..."
special_args() {
  case $1 in 
    s|scanner)
      option_s=()
      for scan in $2; do
        indexOf ${scan}
        if [[ $? == ${#NAMES[@]} ]]; then
          echo -e "${RED}${scan}${RESET_COLOR} is not a valid option for -s. Select from ${GREEN}[${NAMES[@]}]${RESET_COLOR}"
          exit 1
        fi
        option_s+=(${scan})
      done
      ;;
    r|rel)
      cd $2
      if [[ $? != 0 ]]; then
        echo -e "Unable to change directory to ${RED}\"$2\"${RESET_COLOR}!"
        exit 1
      fi
      ;;
    o|out)
      if [[ $2 != "file" && $2 != "dir" ]]; then
        echo -e "${RED}$2${RESET_COLOR} is not a valid option for -o. Must be \"file\" or \"dir\""
        exit 1
      fi
      option_o="$2"
      ;;
    n|name)
      filename="$2"
      ;;
    *)
      indexOf ${1}
      index=$?
      if [[ ${index} == ${#NAMES[@]} ]]; then
        echo -e "${RED}${1}${RESET_COLOR} is not a valid option"
        exit 1
      fi
      FLAGS[${index}]="$2 ${FLAGS[${index}]}"
      ;;
  esac
}

# Start of script logic here 
while (( $# )); do
  case $1 in
    -d|--default)
      option_d=true
      ;;
    -f|--fail)
      option_f=false
      ;;
    -i|--install)
      option_i=true
      ;;
    -I)
      print "${GREEN}Installing dependencies"
      for val in "${SCANNER_INSTALLATION_URL[@]}"; do
        print "${BLUE}Installing:${RESET_COLOR} ${val}"
        eval ${val}
      done
      print "${GREEN}Installation complete!${RESET_COLOR} Exiting"
      exit 0
      ;;
    -q|--quiet)
      option_q=true
      ;;
    -t|--tests)
      option_t=true
      ;;
    -c|--color)
        unset RED
        unset GREEN 
        unset YELLOW
        unset BLUE
        unset MAGENTA
        unset CYAN
        unset RESET_COLOR
        unset BOLD
        unset UNDERLINE
        unset NORMAL
      ;;
    -h|--help)
      echo -e "$help"
      exit 0
      ;;
    -[a-zA-Z0-9]=*)
      suffix=${1#*=}
      prefix=${1%=*}; prefix=${prefix:1}
      special_args "$prefix" "$suffix"
      ;;
    --[a-zA-Z0-9\-]*=*)
      suffix=${1#*=}
      prefix=${1%=*}; prefix=${prefix:2}
      special_args "$prefix" "$suffix"
      ;;
    -*)
      echo -e "${RED}${1}${RESET_COLOR} is not a valid option"
      exit 1
      ;;
    *)
      break
      ;;
  esac
  shift
done

# Append test flags if -t (--test) is set
if [[ ${option_t} == true ]]; then
  for index in ${!FLAGS[@]}; do
    FLAGS[${index}]="${TEST_FLAGS[${index}]} ${FLAGS[${index}]}"
  done
fi

# Checks installation of scanners and install based on provided options:
# $1: command we are running
# $2: installation command
checkInstallation() { 
  if  [[ -x $(command -v $1) ]]; then
    print "${GREEN}Found ${CYAN}$1${RESET_COLOR} executable"
    return 0
  elif [[ ${option_i} == true ]]; then
    print "${YELLOW}Installing ${CYAN}$1${RESET_COLOR} using \"$2\""
    eval $2 
    if  [[ $(command -v $1) ]]; then
      print "${GREEN}Installation Successfull!${RESET_COLOR} continuing"
      return 0
    fi
    print "${BOLD}${RED}Installation Failed!${NORMAL}${RESET_COLOR} There may be something wrong with installation link"
    return 1
  else
    print "${BOLD}${RED}Error!${NORMAL}${RESET_COLOR} ${CYAN}$1${RESET_COLOR} executable not found. Please install or use ${YELLOW}${name} -i${RESET_COLOR} flag"
    return 1
  fi
}

# Run the scans depending on options provide
runScan() {
  local out=0
  # Check if output is terminal or file and create logs accordinly
  if [[ ${option_o} == "" ]]; then
    for package_dir in ${scan_packages}; do
      print "${YELLOW}Scanning${RESET_COLOR}: ${package_dir}/... with command ${GREEN}\"$1 $2 ${package_dir}/...\"${RESET_COLOR}"
      command $1 $2 ./${package_dir}/...
      result=$?
      if [[ ${result} != "0" ]]; then 
        print "${RED}> Fail${RESET_COLOR} ${package_dir} failed $1 scan${RESET_COLOR}"
      else
        print "${GREEN}> Passed${RESET_COLOR} ${package_dir} passed $1 scan${RESET_COLOR}"
      fi
      out=$(( $out + $result )) 
    done
  else
    
    if [[ ${option_o} == "dir" ]]; then
      path_append="/$1.md"
    fi
    tablebuilder="|Package|Result|\n|---|---|\n"
    for package_dir in ${scan_packages}; do
      print "${YELLOW}Scanning${RESET_COLOR}: ${package_dir}/... with command ${GREEN}\"$1 $2 ${package_dir}/...\"${RESET_COLOR}"
      
      echo -e "## Scanning \`${package_dir}\`\n\`\`\`" >> ${filename}${path_append}

      command $1 $2 ./${package_dir}/... >> ${filename}${path_append}
      result=$?
      
      if [[ ${result} == "0" ]]; then
        echo -e "No vulnerabilities found" >> ${filename}${path_append}
      fi

      echo -e "\`\`\`" >> ${filename}${path_append}

      if [[ ${result} != "0" ]]; then 
        print "${RED}> Fail${RESET_COLOR} ${package_dir} failed $1 scan${RESET_COLOR}"
        tablebuilder=${tablebuilder}"|${package_dir}|Failed|\n"
      else
        print "${GREEN}> Passed${RESET_COLOR} ${package_dir} passed $1 scan${RESET_COLOR}"
        tablebuilder=${tablebuilder}"|${package_dir}|Passed|\n"
      fi

      out=$(( $out + $result )) 
    done
    echo -e ${tablebuilder} >> ${filename}${path_append}
  fi
  return ${out}
}

# Find packages
if [[ ${option_d} == true ]]; then
  scan_packages=${DEFAULT_PACKAGES}
elif [[  $# == 0 ]]; then

  # The following line of code performs these steps
  # 1. "git status --porcelain" gets changed files using porcelain as its format 
  # 2. "awk {print $2}" gets the 2nd string from each line
  # 3. "grep \.go" filters out files that do not end in .go
  # 4. "xargs -r 'dirname" gets the directory name associated with each file
  # 5. "sort" sorts output
  # 6. "uniq" Filter out duplicate file locations
  scan_packages=$(git status --porcelain=v1 | awk '{print $2}' | grep '\.go$' | xargs -r 'dirname' | sort | uniq)

  if [[ -z ${scan_packages} ]]; then
    print "${GREEN}No package changes found since last commit${RESET_COLOR}. Specify package paths manually or use ${YELLOW}${name} -d${RESET_COLOR} for defaults locations"
    exit 0
  fi
else
    scan_packages=( $@ )
fi

# Creating logfiles if necessary
if [[ ${option_o} == "file" ]]; then
  print "${MAGENTA}Creating${RESET_COLOR} file ${filename}"
  echo -e "# Vulnerability Report\n>Date: $(date)\n>\n>Locations: [${option_s[@]}] \n>\n>Version: $(go version)" > ${filename}
elif [[ ${option_o} == "dir" ]]; then
  print "${MAGENTA}Creating${RESET_COLOR} directory ./${filename}"
  mkdir -p ${filename}
  for scanner in ${option_s[@]}; do
    echo -e "# ${scanner} report\n>Date: $(date)\n>\n>Locations: [${option_s[@]}] \n>\n>Version: $(go version)" > ./${filename}/${scanner}.md
  done
fi

# Start scanner code
exitcode=0
for scanner in ${option_s[@]}; do
  indexOf ${scanner} 
  index=$?
  print "${BOLD}${GREEN}Starting ${NORMAL}${CYAN}${scanner}${RESET_COLOR} scanner"
  checkInstallation "${scanner}" "${SCANNER_INSTALLATION_URL[${index}]}"
  if [[ $? == 0 ]]; then
    runScan "${scanner}" "${FLAGS[${index}]}"
    result=$?
    print "${GREEN}Completed ${CYAN}${scanner}${RESET_COLOR} scan"
  
    if [[ ${result} != 0 ]]; then
      if [[ ${option_f} == false ]]; then
        print "${RED}Exiting! ${CYAN}${scanner}${RESET_COLOR} found errors"
        exit 1
      else
        print "${RED}Errors${RESET_COLOR} found during ${CYAN}${scanner}${RESET_COLOR} scan"
      fi
    fi

    exitcode=$(( ${result} + ${exitcode} ))

  elif [[ ${option_f} == false ]]; then
    exit 1
  fi
done

# Log messages
print "${GREEN}Completed!${RESET_COLOR} all scans on $(date)"
if [[ ${exitcode} == 0 ]]; then
  echo -e "${GREEN}Success!${RESET_COLOR} no errors found during scanning using $(go version)"
else
  if [[ $option_o == "file" ]]; then
    append="Results are located at ${MAGENTA}${filename}${RESET_COLOR}"
  elif [[ $option_o == "dir" ]]; then
    append="Results are located at ${MAGENTA}./${filename}/${RESET_COLOR}"
  fi
  echo -e "${RED}Errors${RESET_COLOR} where found when scanning using $(go version) ${GREEN}${scan_packages}${RESET_COLOR}. ${append}"
fi

exit ${exitcode}