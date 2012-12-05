package ptx

//This file is auto-generated. Editing is futile.

func init() { Code["madd3"] = MADD3 }

const MADD3 = `
//
// Generated by NVIDIA NVVM Compiler
// Compiler built on Sat Sep 22 02:35:14 2012 (1348274114)
// Cuda compilation tools, release 5.0, V0.2.1221
//

.version 3.1
.target sm_30
.address_size 64

	.file	1 "/tmp/tmpxft_0000372c_00000000-9_madd3.cpp3.i"
	.file	2 "/home/arne/src/code.google.com/p/nimble-cube/gpu/ptx/madd3.cu"

.visible .entry madd3(
	.param .u64 madd3_param_0,
	.param .u64 madd3_param_1,
	.param .f32 madd3_param_2,
	.param .u64 madd3_param_3,
	.param .f32 madd3_param_4,
	.param .u64 madd3_param_5,
	.param .f32 madd3_param_6,
	.param .u32 madd3_param_7
)
{
	.reg .pred 	%p<2>;
	.reg .s32 	%r<13>;
	.reg .f32 	%f<10>;
	.reg .s64 	%rd<14>;


	ld.param.u64 	%rd5, [madd3_param_0];
	ld.param.u64 	%rd6, [madd3_param_1];
	ld.param.f32 	%f1, [madd3_param_2];
	ld.param.u64 	%rd7, [madd3_param_3];
	ld.param.f32 	%f2, [madd3_param_4];
	ld.param.u64 	%rd8, [madd3_param_5];
	ld.param.f32 	%f3, [madd3_param_6];
	ld.param.u32 	%r2, [madd3_param_7];
	cvta.to.global.u64 	%rd1, %rd5;
	cvta.to.global.u64 	%rd2, %rd8;
	cvta.to.global.u64 	%rd3, %rd7;
	cvta.to.global.u64 	%rd4, %rd6;
	.loc 2 4 1
	mov.u32 	%r3, %nctaid.x;
	mov.u32 	%r4, %ctaid.y;
	mov.u32 	%r5, %ctaid.x;
	mad.lo.s32 	%r6, %r3, %r4, %r5;
	mov.u32 	%r7, %ntid.x;
	mov.u32 	%r8, %tid.x;
	mad.lo.s32 	%r1, %r6, %r7, %r8;
	.loc 2 5 1
	setp.ge.s32 	%p1, %r1, %r2;
	@%p1 bra 	BB0_2;

	.loc 2 6 1
	mul.wide.s32 	%rd9, %r1, 4;
	add.s64 	%rd10, %rd4, %rd9;
	ld.global.f32 	%f4, [%rd10];
	add.s64 	%rd11, %rd3, %rd9;
	ld.global.f32 	%f5, [%rd11];
	add.s64 	%rd12, %rd2, %rd9;
	ld.global.f32 	%f6, [%rd12];
	mul.f32 	%f7, %f6, %f3;
	fma.rn.f32 	%f8, %f5, %f2, %f7;
	fma.rn.f32 	%f9, %f4, %f1, %f8;
	add.s64 	%rd13, %rd1, %rd9;
	st.global.f32 	[%rd13], %f9;

BB0_2:
	.loc 2 9 2
	ret;
}


`
