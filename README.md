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
#### Variable definition
To define a variable you use the `var` keyword, followed by the variable name, the type, and an optional default value.

**Examples:**
`var a:int = 5;` - Defines a new variable called 'a' with a type of int and value of 5

`var a:int;` - Also defines a new variable called 'a' with a type of int, but no value. (This is basically useless)