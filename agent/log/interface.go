// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package log

// BasicT represents structs capable of logging messages.
// This interface matches seelog.LoggerInterface.
type BasicT interface {
	// Tracef formats message according to format specifier
	// and writes to log with level Trace.
	Tracef(format string, params ...interface{})

	// Debugf formats message according to format specifier
	// and writes to log with level Debug.
	Debugf(format string, params ...interface{})

	// Infof formats message according to format specifier
	// and writes to log with level Info.
	Infof(format string, params ...interface{})

	// Warnf formats message according to format specifier
	// and writes to log with level Warn.
	Warnf(format string, params ...interface{}) error

	// Errorf formats message according to format specifier
	// and writes to log with level Error.
	Errorf(format string, params ...interface{}) error

	// Criticalf formats message according to format specifier
	// and writes to log with level Critical.
	Criticalf(format string, params ...interface{}) error

	// Trace formats message using the default formats for its operands
	// and writes to log with level Trace.
	Trace(v ...interface{})

	// Debug formats message using the default formats for its operands
	// and writes to log with level Debug.
	Debug(v ...interface{})

	// Info formats message using the default formats for its operands
	// and writes to log with level Info.
	Info(v ...interface{})

	// Warn formats message using the default formats for its operands
	// and writes to log with level Warn.
	Warn(v ...interface{}) error

	// Error formats message using the default formats for its operands
	// and writes to log with level Error.
	Error(v ...interface{}) error

	// Critical formats message using the default formats for its operands
	// and writes to log with level Critical.
	Critical(v ...interface{}) error

	// Flush flushes all the messages in the logger.
	Flush()

	// Close flushes all the messages in the logger and closes it. The logger cannot be used after this operation.
	Close()
}

// T represents structs capable of logging messages, and context management.
type T interface {
	BasicT
	WithContext(context ...string) (contextLogger T)
}
