@a = global i64 5
@format = global [3 x i8] c"%d\0A"

define void @main() {
0:
	%1 = call i32 @printf([3 x i8]* @format, i64 5)
	ret void
}

declare i32 @printf(i8* %0)

declare void @sleep(i64 %0)
