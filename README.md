# CaffeineC
Idk why I'm doing this, so don't ask.
## Building the compiler
It's pretty easy, you need to have Golang installed though.

Run `go build ./src` in the project's root directory to build the executable.

Once you have the executable you use it like this: `./CaffeineC <file_to_compile>`.

If everything goes well, you should be left with a executable file named `output`.
## Troubleshooting
This has really only been tested on linux.

Make sure every command is being executed in the project's root directory.

## Docs (sort of)
### Compiler use
The compiler command is very basic, only offering 1 subcommand and 2 options (at this time).

Also, you can ignore most of the compiler's output.
If you don't see a panic, or this line: `2023/11/20 09:31:53 exit status 1`, it probably worked.
| Subcommand | Description | Arguments | Options |
| --- | --- | --- | --- |
| build | Builds the supplied CaffeineC program | <file to build> - Required | -nc, -nv |

| Option | Type | Description |
| --- | --- | --- |
| -nc | bool | No Clean - prevents the compiler for clearing generated files (.ll and .o) |
| -nv | bool | Numbers as variable names - allows you to use numbers as variable names |

### CaffeineC Syntax
The syntax of the CaffeineC syntax is a mix of many other programming languages.

In CaffeineC, all the code you write will at compile time be put in a main function automatically.
#### Variable definition
To define a variable you use the `var` keyword, followed by the variable name, the type, and an optional default value.

**Examples:**

`var a:int = 5;` - Defines a new variable called 'a' with a type of int and value of 5

`var a:int;` - Also defines a new variable called 'a' with a type of int, but no value. (This is basically useless)

##### Using with -nv option
As stated above, when using the `-nv` option, the compiler allows you to use numbers as variable names. This makes the following examples valid.

**Examples with -nv:**

`var 1:int = 5;` - Defines a new variable called '1' with a value of 5

`var 10:int = 1;` - If used with the line above, defines a variable called '10' with a value of 5, if not, the value will be 1

#### Variable assignment
The variable assignment is implemented, however it doesn't work yet (idk why, I'll look into it).
Anyway the syntax should be fairly simple and similair to other langauges.

`a = 5;`

#### Print statement
The CaffeineC programming language also has a builtin print statement. 
It works very similairly to python2. 
This function takes exactly one argument.
Internally this uses the `printf` function from C, with a automatically generated format string.

`print "a"` - Prints 'a' to the console.

#### Sleep statement
We also have a sleep statement that works similairly to the print statement.
It also takes one argument, the duration stating how long to sleep for.
This can be a number (in which case it is understood as a duration in nanoseconds),
or a value of the custom 'duration' type, which is just a number with a duration suffix
(one of ns/us/ms/s/m/h). This uses the sleep_ns function from the `./c_files/sleep.c` file

`sleep 500ms` - Sleeps for 500ms

`sleep 500000000` - Also sleeps for 500ms (but why would you use this)

#### Defining functions
Function defining is also pretty easy. You use the `func` keyword followed by the function name,
then the list of arguments and their types in the `name:type` format, lastly the optional type
(defaults to void) followed by the curly braces.

For these functions (and the main function) there is also a `return` statement.
The return state work as expected, but with one exception from the rest of the language,
you don't put a semicolon at the end ._.

**Example:**

```
func add(a: int, b:int):int {
  return a + b
}

print add(5, 10);
```

#### Calling functions
Nothing special here.

`add(5, 10);`

#### If/Else statements
Two important things here. First, there is no if else (or elif), and second, you have to use both if and else. If you leave out else after an if, it doesn't work.
Also, there is no boolean type yet.

**Example:**
```
if 1 {
  print "True";
} else {
  print "False";
}
```

**Interesting observation**
Due to the way that the condition handling is implemented, if you supply a number different than 0 or 1 to the if statement, than even numbers are taken as false and odd numbers as true.