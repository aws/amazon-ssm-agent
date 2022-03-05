#!/bin/sh
#
# Copyright 2019 The Mangos Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use file except in compliance with the License.
# You may obtain a copy of the license at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

url=tcp://127.0.0.1:40899
go run pipeline.go node0 $url
sleep 1
go run pipeline.go node1 $url "Hello, World."
go run pipeline.go node1 $url "Goodbye."
go run pipeline.go node1 $url "STOP"
