// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package registry

var (
	startMarker        = "<start" + randomString(8) + ">"
	endMarker          = "<end" + randomString(8) + ">"
	registryInfoScript = `
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
	$global:count = 0
	$global:valueCount = 0
	$global:registryKeys = @()

	function Get-Type-Name($typeName) {
	  switch($typeName) {
	     "Binary"  { return "REG_BINARY" }
	     "DWord" { return "REG_DWORD" }
	     "ExpandString" { "REG_EXPAND_SZ" }
	     "MultiString" { return "REG_MULTI_SZ" }
	     "None" { return "None" }
	     "QWord" { return "REG_QWORD" }
	     "String" { return "REG_SZ" }
	     "Unknown" { return "Unknown" }
	      default {return $typeName }
	  }
	}

    function Get-RegistryValue($key, $valueName) {
        $value = $key.GetValue($valueName)
        if ($value -ne $null) {
	        $valueType = $key.GetValueKind($valueName)
	        if ($valueType.toString() -eq "Binary") {
	            $value = "BinaryValue"
	        }
	        $valueTypeName = Get-Type-Name $valueType
	        $keyName = $key.Name
	        $regJson =  @"
{"KeyPath":"` + mark(`$keyName`) + `","Value":"` + mark(`$value`) + `","ValueName":"` + mark(`$valueName`) + `","ValueType":"$valueTypeName"}
"@
	        $global:registryKeys += $regJson
		    $global:valueCount = $global:valueCount + 1
        }

    }


	function Get-RegistryKeys ($key, $valueLimit, $Recursive) {
	   try {
	       $global:count = $global:count + 1

	       $subKeys = $key.GetSubKeyNames();
	       $valueNames = $key.GetValueNames();
	       foreach ($valueName in $valueNames) {
		        if ($global:valueCount -gt $valueLimit) {
					return;
				}
                Get-RegistryValue $key $valueName

	       }

	       if ($Recursive) {
	           foreach ($sub in $subKeys) {
			      if ($global:valueCount -gt $valueLimit) {
				    return;
			      }
	              try {

	                   $subKey = $key.OpenSubKey($sub)
                       Get-RegistryKeys $subKey $valueLimit $Recursive


	              } catch {
	                	Write-Error $_.Exception.Message
	              }

	           }
	       }
	   } catch {
	      Write-Error $_.Exception.Message
	   } finally {
	     $key.close()
	   }


	}

	function Get-RegistryKeysFromPath($path, $valueLimit, [switch]$Recursive, [String[]]$Values) {
		try {
            $keyExists = Test-Path $path
            if ($keyExists) {
                $key = Get-Item $path

                if($Values) {
                   foreach($valueName in $values) {
				   	if ($global:valueCount -gt $valueLimit) {
					   break;
				   	}
                    Get-RegistryValue $key $valueName
                   }

                } else {
                    Get-RegistryKeys $key $valueLimit $Recursive

                }
				if ($global:valueCount -gt $valueLimit) {
					Write-Output "ValueLimitExceeded"
				} else {
	                $result = $global:registryKeys -join ","
			        $result = "[" + $result + "]"
			        [Console]::WriteLine($result)
				}
            } else {
                Write-Output "[]"
            }
	    } catch {
		    Write-Error $_.Exception.Message
	    }

	}

	Get-RegistryKeysFromPath `
)

func mark(s string) string {
	return startMarker + s + endMarker
}
