package subagent

import "time"

// Result is what a subagent returns after execution 
type Result struct{
	ID 				string    		  // Subagent ID
	Summary 		string 			  // Final Text Response of Subagent
	Success 		bool 			  // if the task was successful or not
	Error 			string  		  // error messages if failed
	ToolCalls 		int               // Number of Tool calls conducted
	Duration 		time.Duration     // Duration is how long the subagent ran
	FilesModified 	[]string          // list of files that were changed
}