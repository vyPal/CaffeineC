@greeting = global [14 x i8] c"\22Hello, World\22"
@format = global [3 x i8] c"%d\0A"
@a = global i64 5
@b = global i64 10
@sum = global i64 add (i64 5, i64 10)

define void @main() {
0:
	%1 = call i32 @printf([3 x i8]* @format, [14 x i8] c"\22Hello, World\22")
	%2 = call i32 @printf([3 x i8]* @format, i64 add (i64 5, i64 10))
	ret void
}

declare i32 @printf(i8* %0)

declare void @sleep(i64 %0)
