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

import (
	"sync"
)

// DelegateLogger holds the base logger for logging
type DelegateLogger struct {
	BaseLoggerInstance BasicT
}

// Wrapper is a logger that can modify the format of a log message before delegating to another logger.
type Wrapper struct {
	Format   FormatFilter
	M        *sync.Mutex
	Delegate *DelegateLogger
}

// FormatFilter can modify the format and or parameters to be passed to a logger.
type FormatFilter interface {

	// Filter modifies parameters that will be passed to log.Debug, log.Info, etc.
	Filter(params ...interface{}) (newParams []interface{})

	// Filter modifies format and/or parameter strings that will be passed to log.Debugf, log.Infof, etc.
	Filterf(format string, params ...interface{}) (newFormat string, newParams []interface{})
}

// WithContext creates a wrapper logger with context
func (w *Wrapper) WithContext(context ...string) (contextLogger T) {
	formatFilter := &ContextFormatFilter{Context: context}
	contextLogger = &Wrapper{Format: formatFilter, M: w.M, Delegate: w.Delegate}
	return contextLogger
}

// Tracef formats message according to format specifier
// and writes to log with level = Trace.
func (w *Wrapper) Tracef(format string, params ...interface{}) {
	format, params = w.Format.Filterf(format, params...)

	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Tracef(format, params...)
}

// Debugf formats message according to format specifier
// and writes to log with level = Debug.
func (w *Wrapper) Debugf(format string, params ...interface{}) {
	format, params = w.Format.Filterf(format, params...)

	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Debugf(format, params...)
}

// Infof formats message according to format specifier
// and writes to log with level = Info.
func (w *Wrapper) Infof(format string, params ...interface{}) {
	format, params = w.Format.Filterf(format, params...)

	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Infof(format, params...)
}

// Warnf formats message according to format specifier
// and writes to log with level = Warn.
func (w *Wrapper) Warnf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Warnf(format, params...)
}

// Errorf formats message according to format specifier
// and writes to log with level = Error.
func (w *Wrapper) Errorf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Errorf(format, params...)
}

// Criticalf formats message according to format specifier
// and writes to log with level = Critical.
func (w *Wrapper) Criticalf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Criticalf(format, params...)
}

// Trace formats message using the default formats for its operands
// and writes to log with level = Trace
func (w *Wrapper) Trace(v ...interface{}) {
	v = w.Format.Filter(v...)
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Trace(v...)
}

// Debug formats message using the default formats for its operands
// and writes to log with level = Debug
func (w *Wrapper) Debug(v ...interface{}) {
	v = w.Format.Filter(v...)

	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Debug(v...)
}

// Info formats message using the default formats for its operands
// and writes to log with level = Info
func (w *Wrapper) Info(v ...interface{}) {
	v = w.Format.Filter(v...)

	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Info(v...)
}

// Warn formats message using the default formats for its operands
// and writes to log with level = Warn
func (w *Wrapper) Warn(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Warn(v...)
}

// Error formats message using the default formats for its operands
// and writes to log with level = Error
func (w *Wrapper) Error(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Error(v...)
}

// Critical formats message using the default formats for its operands
// and writes to log with level = Critical
func (w *Wrapper) Critical(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Critical(v...)
}

// Flush flushes all the messages in the logger.
func (w *Wrapper) Flush() {
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Flush()
}

// Close flushes all the messages in the logger and closes it. It cannot be used after this operation.
func (w *Wrapper) Close() {
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Close()
}

// ReplaceDelegate replaces the delegate logger with a new logger
func (w *Wrapper) ReplaceDelegate(newLogger BasicT) {
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Flush()
	w.Delegate.BaseLoggerInstance = newLogger
	w.Delegate.BaseLoggerInstance.Info("Logger Replaced. New Logger Used to log the message")
}
